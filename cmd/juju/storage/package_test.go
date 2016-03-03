// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package storage_test

import (
	"os"
	"testing"

	"github.com/juju/cmd"
	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"

	"github.com/juju/juju/cmd/juju/storage"
	"github.com/juju/juju/environs/configstore"
	"github.com/juju/juju/juju/osenv"
	jujutesting "github.com/juju/juju/testing"
)

func TestAll(t *testing.T) {
	gc.TestingT(t)
}

type BaseStorageSuite struct {
	jujutesting.FakeJujuXDGDataHomeSuite

	command cmd.Command
}

func (s *BaseStorageSuite) SetUpTest(c *gc.C) {
	s.FakeJujuXDGDataHomeSuite.SetUpTest(c)

	s.command = storage.NewSuperCommand()
}

func (s *BaseStorageSuite) TearDownTest(c *gc.C) {
	s.FakeJujuXDGDataHomeSuite.TearDownTest(c)
}

type SubStorageSuite struct {
	jujutesting.FakeJujuXDGDataHomeSuite
}

func (s *SubStorageSuite) SetUpTest(c *gc.C) {
	s.FakeJujuXDGDataHomeSuite.SetUpTest(c)

	memstore := configstore.NewMem()
	s.PatchValue(&configstore.Default, func() (configstore.Storage, error) {
		return memstore, nil
	})
	os.Setenv(osenv.JujuModelEnvKey, "testing")
	info := memstore.CreateInfo("testing")
	info.SetBootstrapConfig(map[string]interface{}{"random": "extra data"})
	info.SetAPIEndpoint(configstore.APIEndpoint{
		Addresses: []string{"127.0.0.1:12345"},
		Hostnames: []string{"localhost:12345"},
		CACert:    jujutesting.CACert,
		ModelUUID: jujutesting.ModelTag.Id(),
	})
	info.SetAPICredentials(configstore.APICredentials{
		User:     "user-test",
		Password: "password",
	})
	err := info.Write()
	c.Assert(err, jc.ErrorIsNil)
}
