package usecase

// AuthConfig tells the login screen what the instance offers. It holds
// nothing secret, as everyone may read it.
type AuthConfig struct {
	OIDCEnabled  bool
	LocalLogin   bool
	InstanceName string
	Version      string
}

// GetAuthConfig returns the login configuration of the instance. It takes
// no principal: the login screen asks before anyone has signed in.
type GetAuthConfig struct {
	oidc OIDCProvider
	cfg  AuthConfig
}

func NewGetAuthConfig(oidc OIDCProvider, localLogin bool, instanceName, version string) *GetAuthConfig {
	return &GetAuthConfig{
		oidc: oidc,
		cfg:  AuthConfig{LocalLogin: localLogin, InstanceName: instanceName, Version: version},
	}
}

// Do asks the provider itself whether OIDC is on, so the answer cannot
// drift from what the login does.
func (uc *GetAuthConfig) Do() AuthConfig {
	cfg := uc.cfg
	cfg.OIDCEnabled = uc.oidc.Enabled()
	return cfg
}
