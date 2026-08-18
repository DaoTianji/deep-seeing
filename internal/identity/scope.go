package identity

// MaxTenantFieldLen caps UserID / AgentID length.
const MaxTenantFieldLen = 128

// ValidationError is returned for invalid tenant fields.
type ValidationError struct {
	Msg string
}

func (e ValidationError) Error() string { return e.Msg }

// TenantScope identifies a user and agent. Memory paths must carry this scope.
type TenantScope struct {
	UserID  string
	AgentID string
}

// LocalCLI is the default scope for cmd/see (human counterpart: mudnet).
func LocalCLI() TenantScope {
	return TenantScope{UserID: "mudnet", AgentID: "deep-seeing"}
}

// PersonID returns the stable person key for this scope's human.
func (s TenantScope) PersonID() string {
	return "user:" + s.UserID
}

// Validate returns an error if required fields are empty or oversized.
func (s TenantScope) Validate() error {
	if s.UserID == "" {
		return ValidationError{Msg: "user_id is required"}
	}
	if len(s.UserID) > MaxTenantFieldLen {
		return ValidationError{Msg: "user_id is too long"}
	}
	if s.AgentID == "" {
		return ValidationError{Msg: "agent_id is required"}
	}
	if len(s.AgentID) > MaxTenantFieldLen {
		return ValidationError{Msg: "agent_id is too long"}
	}
	return nil
}
