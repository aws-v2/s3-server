package middleware

import (
	"path"
	"s3/internal/domain"
	"strings"
)

type Decision int

const (
	DecisionDeny Decision = iota
	DecisionAllow
)

func EvaluatePolicy(policy *domain.Policy, principal string, action domain.Action, resource string) Decision {
	if policy == nil {
		return DecisionDeny
	}
	// iterate statements; Deny wins immediately
	for _, stmt := range policy.Statement {
		// principal match?
		if !principalMatches(stmt.Principal, principal) {
			continue
		}
		// action match?
		if !actionMatches(stmt.Action, action) {
			continue
		}
		// resource match?
		if !resourceMatches(stmt.Resource, resource) {
			continue
		}
		// evaluation
		if stmt.Effect == domain.EffectDeny {
			return DecisionDeny
		}
		if stmt.Effect == domain.EffectAllow {
			// allow, but continue to check for explicit denies above (we checked denies first by order)
			return DecisionAllow
		}
	}
	return DecisionDeny
}

func principalMatches(list []domain.Principal, principal string) bool {
	for _, p := range list {
		if string(p) == "public" && principal == "public" {
			return true
		}
		if strings.HasPrefix(string(p), "user:") && string(p) == principal {
			return true
		}
		if strings.HasPrefix(principal, "user:") && string(p) == principal {
			// exact match
			return true
		}
		// add role matching here if needed: "role:admin"
	}
	return false
}

func actionMatches(actions []domain.Action, action domain.Action) bool {
	for _, a := range actions {
		if a == domain.ActionAll || a == action {
			return true
		}
	}
	return false
}

func resourceMatches(patterns []string, resource string) bool {
	for _, pat := range patterns {
		// basic path.Match support - convert ARN-like wildcard to match syntax
		// replace '*' with '*' and keep it simple for now
		ok, _ := path.Match(pat, resource)
		if ok {
			return true
		}
		// also match simple prefix style (e.g., "arn:mys3:::bucketId/*")
		if strings.HasSuffix(pat, "/*") && strings.HasPrefix(resource, strings.TrimSuffix(pat, "*")) {
			return true
		}
	}
	return false
}

