package usecase_test

import "github.com/paramah/denar/internal/usecase/fakes"

type fakeOIDC = fakes.OIDCProvider

func newFakeOIDC() *fakeOIDC { return fakes.NewOIDCProvider() }
