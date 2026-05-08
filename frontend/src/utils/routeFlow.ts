import type { RouteGroup, RouteRule } from '../data/demoData';

export function canEnableRouteGroupSource(
  groups: RouteGroup[],
  currentGroupId: string | null,
  sourceCode: string,
) {
  return !groups.some(
    (group) => group.enabled && group.sourceCode === sourceCode && group.id !== currentGroupId,
  );
}

export function routeRulesForGroup(group: RouteGroup, rules: RouteRule[]) {
  return rules
    .filter((rule) => group.ruleIds.includes(rule.id))
    .sort((left, right) => left.sortOrder - right.sortOrder);
}
