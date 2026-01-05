package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Action is strongly-typed action values
type Action string

const (
	ActionGetObject    Action = "s3:GetObject"
	ActionPutObject    Action = "s3:PutObject"
	ActionDeleteObject Action = "s3:DeleteObject"
	ActionListBucket   Action = "s3:ListBucket"
	ActionAll          Action = "s3:*"
)

type Effect string

const (
	EffectAllow Effect = "Allow"
	EffectDeny  Effect = "Deny"
)

// Principal can be "user:<id>", "role:<name>", "public"
type Principal string

// Statement is a single policy statement
type Statement struct {
	Effect    Effect     `json:"Effect"`
	Principal []Principal `json:"Principal"`
	Action    []Action   `json:"Action"`
	Resource  []string   `json:"Resource"` // use patterns, e.g., "arn:mys3:::bucketId/*"
	Condition map[string]interface{}  `json:"condition,omitempty"`
}

// Policy is the container for statements
type Policy struct {
	Version   string      `json:"Version"`
	Statement []Statement `json:"Statement"`
	CreatedAt time.Time   `json:"-"`
}

// Validate performs basic structural validation; extend as needed or use JSON Schema
func (p *Policy) Validate() error {
	if p == nil {
		return errors.New("policy is nil")
	}
	if strings.TrimSpace(p.Version) == "" {
		return errors.New("policy version required")
	}
	if len(p.Statement) == 0 {
		return errors.New("at least one statement required")
	}
	for i, s := range p.Statement {
		if s.Effect != EffectAllow && s.Effect != EffectDeny {
			return fmt.Errorf("statement[%d]: invalid effect: %s", i, s.Effect)
		}
		if len(s.Principal) == 0 {
			return fmt.Errorf("statement[%d]: principal required", i)
		}
		if len(s.Action) == 0 {
			return fmt.Errorf("statement[%d]: action required", i)
		}
		if len(s.Resource) == 0 {
			return fmt.Errorf("statement[%d]: resource required", i)
		}
	}
	return nil
}

func (p *Policy) String() string {
	if p == nil {
		return "<nil>"
	}
	b, _ := json.MarshalIndent(p, "", "  ")
	return string(b)
}
