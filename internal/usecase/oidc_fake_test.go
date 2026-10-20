package usecase_test

import "git.eiac.dev/tools/denar/internal/usecase/fakes"

type fakeOIDC = fakes.OIDCProvider

func newFakeOIDC() *fakeOIDC { return fakes.NewOIDCProvider() }
