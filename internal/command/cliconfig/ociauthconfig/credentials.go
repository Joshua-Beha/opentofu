// Copyright (c) The OpenTofu Authors
// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2023 HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package ociauthconfig

import (
	orasauth "oras.land/oras-go/v2/registry/remote/auth"
)

type Credentials struct {
	// The internals are intentionally not exposed so that we can extend this with
	// other types of credentials in future, if needed.

	// username and password are used for the "Basic" auth scheme, and possibly
	// others that use username and password.
	username, password string

	// accessToken and refreshToken are used for OAuth-style authentication, with
	// the accessToken acting as a Bearer token for direct authentication but the
	// refreshToken used only to renew an expired access token.
	accessToken, refreshToken string
}

// ForORAS converts the credentials into the type expected by the ORAS v2 Go
// library, which we use to interact with OCI registries.
func (c *Credentials) ForORAS() orasauth.Credential {
	return orasauth.Credential{
		Username:     c.username,
		Password:     c.password,
		AccessToken:  c.accessToken,
		RefreshToken: c.refreshToken,
	}
}
