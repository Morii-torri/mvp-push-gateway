import { apiRequest, tokenStore, type ApiFetcher } from "./client";

export type AdminUser = {
  id: string;
  username: string;
  display_name: string;
  must_change_password: boolean;
  enabled: boolean;
};

export type SetupStatus = {
  initialized: boolean;
  setup_open: boolean;
  admin_count: number;
};

export type LoginResult = {
  token: string;
  token_type: string;
  expires_at: string;
  admin: AdminUser;
};

export const authApi = {
  getSetupStatus(fetcher?: ApiFetcher) {
    return apiRequest<SetupStatus>("/setup/status", { auth: false, fetcher });
  },
  setupAdmin(
    input: {
      username: string;
      password: string;
      confirm_password: string;
      display_name: string;
    },
    fetcher?: ApiFetcher,
  ) {
    return apiRequest<{ admin: AdminUser }>("/setup/admin", {
      method: "POST",
      body: input,
      auth: false,
      fetcher,
    });
  },
  async login(
    input: { username: string; password: string },
    fetcher?: ApiFetcher,
  ) {
    const result = await apiRequest<LoginResult>("/auth/login", {
      method: "POST",
      body: input,
      auth: false,
      fetcher,
    });
    tokenStore.set(result.token);
    return result;
  },
  me(fetcher?: ApiFetcher) {
    return apiRequest<{ admin: AdminUser }>("/auth/me", { fetcher });
  },
  updateProfile(input: { display_name: string }, fetcher?: ApiFetcher) {
    return apiRequest<{ admin: AdminUser }>("/auth/profile", {
      method: "PUT",
      body: input,
      fetcher,
    });
  },
  changePassword(
    input: { current_password: string; new_password: string },
    fetcher?: ApiFetcher,
  ) {
    return apiRequest<{ ok: boolean }>("/auth/change-password", {
      method: "POST",
      body: input,
      fetcher,
    });
  },
  async logout(fetcher?: ApiFetcher) {
    try {
      await apiRequest<{ ok: boolean }>("/auth/logout", {
        method: "POST",
        fetcher,
      });
    } finally {
      tokenStore.clear();
    }
  },
};
