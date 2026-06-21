package httpapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	captchaTTL        = 2 * time.Minute
	captchaCodeLength = 6
)

var captchaAlphabet = []rune("ABCDEFGHJKLMNPQRSTUVWXYZ23456789")

type captchaResponse struct {
	CaptchaID        string `json:"captcha_id"`
	ImageDataURL     string `json:"image_data_url"`
	ExpiresInSeconds int    `json:"expires_in_seconds"`
}

type captchaChallenge struct {
	answerHash [32]byte
	expiresAt  time.Time
}

type LoginCaptchaStateStore interface {
	StoreLoginCaptcha(ctx context.Context, id string, answerHash [32]byte, expiresAt time.Time) error
	ConsumeLoginCaptcha(ctx context.Context, id string, answerHash [32]byte, now time.Time) (bool, error)
}

type captchaStore struct {
	ttl            time.Duration
	state          LoginCaptchaStateStore
	answerOverride string
	now            func() time.Time
}

type memoryCaptchaStateStore struct {
	mu         sync.Mutex
	challenges map[string]captchaChallenge
}

func newCaptchaStore() *captchaStore {
	return &captchaStore{
		ttl:   captchaTTL,
		state: newMemoryCaptchaStateStore(),
		now:   time.Now,
	}
}

func newFixedCaptchaStoreForTesting(answer string) *captchaStore {
	store := newCaptchaStore()
	store.answerOverride = normalizeCaptchaAnswer(answer)
	return store
}

func newMemoryCaptchaStateStore() *memoryCaptchaStateStore {
	return &memoryCaptchaStateStore{challenges: map[string]captchaChallenge{}}
}

func (h *Handler) captchaHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if h.loginCaptcha == nil {
		h.loginCaptcha = newCaptchaStore()
	}

	challenge, err := h.loginCaptcha.newChallenge(r.Context())
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "MGP-AUTH-999", "认证服务内部错误")
		return
	}
	imageBytes, err := renderCaptchaPNG(challenge.answer)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "MGP-AUTH-999", "认证服务内部错误")
		return
	}

	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	writeJSON(w, http.StatusOK, captchaResponse{
		CaptchaID:        challenge.id,
		ImageDataURL:     "data:image/png;base64," + base64.StdEncoding.EncodeToString(imageBytes),
		ExpiresInSeconds: int(h.loginCaptcha.ttl.Seconds()),
	})
}

type issuedCaptcha struct {
	id     string
	answer string
}

func (s *captchaStore) newChallenge(ctx context.Context) (issuedCaptcha, error) {
	if s == nil {
		s = newCaptchaStore()
	}
	if s.state == nil {
		s.state = newMemoryCaptchaStateStore()
	}
	id, err := randomCaptchaID()
	if err != nil {
		return issuedCaptcha{}, err
	}
	answer, err := s.newAnswer()
	if err != nil {
		return issuedCaptcha{}, err
	}
	now := s.now()
	expiresAt := now.Add(s.ttl)
	if err := s.state.StoreLoginCaptcha(ctx, id, captchaAnswerHash(id, answer), expiresAt); err != nil {
		return issuedCaptcha{}, err
	}
	return issuedCaptcha{id: id, answer: answer}, nil
}

func (s *captchaStore) verify(ctx context.Context, id string, answer string) (bool, error) {
	if s == nil {
		return false, nil
	}
	id = strings.TrimSpace(id)
	answer = normalizeCaptchaAnswer(answer)
	if id == "" || answer == "" {
		return false, nil
	}
	if s.state == nil {
		s.state = newMemoryCaptchaStateStore()
	}

	return s.state.ConsumeLoginCaptcha(ctx, id, captchaAnswerHash(id, answer), s.now())
}

func (s *memoryCaptchaStateStore) StoreLoginCaptcha(ctx context.Context, id string, answerHash [32]byte, expiresAt time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked(now)
	s.challenges[id] = captchaChallenge{
		answerHash: answerHash,
		expiresAt:  expiresAt,
	}
	return nil
}

func (s *memoryCaptchaStateStore) ConsumeLoginCaptcha(ctx context.Context, id string, answerHash [32]byte, now time.Time) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return false, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked(now)
	challenge, ok := s.challenges[id]
	if !ok {
		return false, nil
	}
	delete(s.challenges, id)
	if now.After(challenge.expiresAt) {
		return false, nil
	}
	return subtle.ConstantTimeCompare(answerHash[:], challenge.answerHash[:]) == 1, nil
}

func (s *memoryCaptchaStateStore) pruneLocked(now time.Time) {
	for id, challenge := range s.challenges {
		if !now.Before(challenge.expiresAt) {
			delete(s.challenges, id)
		}
	}
}

func (s *captchaStore) newAnswer() (string, error) {
	if s.answerOverride != "" {
		return s.answerOverride, nil
	}
	runes := make([]rune, captchaCodeLength)
	for index := range runes {
		position, err := cryptoRandInt(len(captchaAlphabet))
		if err != nil {
			return "", err
		}
		runes[index] = captchaAlphabet[position]
	}
	return string(runes), nil
}

func randomCaptchaID() (string, error) {
	buf := make([]byte, 18)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func normalizeCaptchaAnswer(answer string) string {
	return strings.ToUpper(strings.TrimSpace(answer))
}

func captchaAnswerHash(id string, answer string) [32]byte {
	return sha256.Sum256([]byte(strings.TrimSpace(id) + "\x00" + normalizeCaptchaAnswer(answer)))
}

func renderCaptchaPNG(answer string) ([]byte, error) {
	const (
		width  = 184
		height = 58
		scale  = 4
	)
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{R: 246, G: 250, B: 255, A: 255}}, image.Point{}, draw.Src)

	for i := 0; i < 9; i++ {
		x1, err := cryptoRandInt(width)
		if err != nil {
			return nil, err
		}
		y1, err := cryptoRandInt(height)
		if err != nil {
			return nil, err
		}
		x2, err := cryptoRandInt(width)
		if err != nil {
			return nil, err
		}
		y2, err := cryptoRandInt(height)
		if err != nil {
			return nil, err
		}
		drawLine(img, x1, y1, x2, y2, color.RGBA{R: 115, G: 161, B: 226, A: 95})
	}

	for i := 0; i < 220; i++ {
		x, err := cryptoRandInt(width)
		if err != nil {
			return nil, err
		}
		y, err := cryptoRandInt(height)
		if err != nil {
			return nil, err
		}
		img.Set(x, y, color.RGBA{R: 61, G: 126, B: 215, A: 70})
	}

	startX := 14
	for index, char := range answer {
		pattern, ok := captchaGlyphs[char]
		if !ok {
			continue
		}
		jitterX, err := cryptoRandInt(5)
		if err != nil {
			return nil, err
		}
		jitterY, err := cryptoRandInt(7)
		if err != nil {
			return nil, err
		}
		x := startX + index*27 + jitterX - 2
		y := 12 + jitterY - 3
		ink := color.RGBA{R: 25, G: 91, B: 190, A: 225}
		if index%2 == 1 {
			ink = color.RGBA{R: 20, G: 123, B: 218, A: 225}
		}
		drawCaptchaGlyph(img, pattern, x, y, scale, ink)
	}

	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func cryptoRandInt(max int) (int, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, err
	}
	return int(n.Int64()), nil
}

func drawCaptchaGlyph(img *image.RGBA, pattern []string, x int, y int, scale int, ink color.Color) {
	for row, line := range pattern {
		for col, cell := range line {
			if cell != '1' {
				continue
			}
			rect := image.Rect(x+col*scale, y+row*scale, x+(col+1)*scale-1, y+(row+1)*scale-1)
			draw.Draw(img, rect, &image.Uniform{C: ink}, image.Point{}, draw.Over)
		}
	}
}

func drawLine(img *image.RGBA, x0 int, y0 int, x1 int, y1 int, ink color.Color) {
	dx := absInt(x1 - x0)
	dy := -absInt(y1 - y0)
	sx := -1
	if x0 < x1 {
		sx = 1
	}
	sy := -1
	if y0 < y1 {
		sy = 1
	}
	err := dx + dy
	for {
		if image.Pt(x0, y0).In(img.Bounds()) {
			img.Set(x0, y0, ink)
		}
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

var captchaGlyphs = map[rune][]string{
	'A': {"01110", "10001", "10001", "11111", "10001", "10001", "10001"},
	'B': {"11110", "10001", "10001", "11110", "10001", "10001", "11110"},
	'C': {"01111", "10000", "10000", "10000", "10000", "10000", "01111"},
	'D': {"11110", "10001", "10001", "10001", "10001", "10001", "11110"},
	'E': {"11111", "10000", "10000", "11110", "10000", "10000", "11111"},
	'F': {"11111", "10000", "10000", "11110", "10000", "10000", "10000"},
	'G': {"01111", "10000", "10000", "10011", "10001", "10001", "01110"},
	'H': {"10001", "10001", "10001", "11111", "10001", "10001", "10001"},
	'J': {"00111", "00010", "00010", "00010", "10010", "10010", "01100"},
	'K': {"10001", "10010", "10100", "11000", "10100", "10010", "10001"},
	'L': {"10000", "10000", "10000", "10000", "10000", "10000", "11111"},
	'M': {"10001", "11011", "10101", "10101", "10001", "10001", "10001"},
	'N': {"10001", "11001", "10101", "10011", "10001", "10001", "10001"},
	'P': {"11110", "10001", "10001", "11110", "10000", "10000", "10000"},
	'Q': {"01110", "10001", "10001", "10001", "10101", "10010", "01101"},
	'R': {"11110", "10001", "10001", "11110", "10100", "10010", "10001"},
	'S': {"01111", "10000", "10000", "01110", "00001", "00001", "11110"},
	'T': {"11111", "00100", "00100", "00100", "00100", "00100", "00100"},
	'U': {"10001", "10001", "10001", "10001", "10001", "10001", "01110"},
	'V': {"10001", "10001", "10001", "10001", "10001", "01010", "00100"},
	'W': {"10001", "10001", "10001", "10101", "10101", "11011", "10001"},
	'X': {"10001", "10001", "01010", "00100", "01010", "10001", "10001"},
	'Y': {"10001", "10001", "01010", "00100", "00100", "00100", "00100"},
	'Z': {"11111", "00001", "00010", "00100", "01000", "10000", "11111"},
	'2': {"01110", "10001", "00001", "00010", "00100", "01000", "11111"},
	'3': {"11110", "00001", "00001", "01110", "00001", "00001", "11110"},
	'4': {"00010", "00110", "01010", "10010", "11111", "00010", "00010"},
	'5': {"11111", "10000", "10000", "11110", "00001", "00001", "11110"},
	'6': {"01111", "10000", "10000", "11110", "10001", "10001", "01110"},
	'7': {"11111", "00001", "00010", "00100", "01000", "01000", "01000"},
	'8': {"01110", "10001", "10001", "01110", "10001", "10001", "01110"},
	'9': {"01110", "10001", "10001", "01111", "00001", "00001", "11110"},
}
