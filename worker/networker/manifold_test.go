// Copyright 2016 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package networker_test

import (
	apinetworker "github.com/juju/juju/api/networker"
	"github.com/juju/names"
	jc "github.com/juju/testing/checkers"
	"github.com/waigani/xxx"
	gc "gopkg.in/check.v1"

	"github.com/juju/juju/agent"
	basetesting "github.com/juju/juju/api/base/testing"
	"github.com/juju/juju/state/multiwatcher"
	coretesting "github.com/juju/juju/testing"
	"github.com/juju/juju/worker"
	"github.com/juju/juju/worker/networker"
)

type manifoldSuite struct {
	coretesting.BaseSuite
}

var _ = gc.Suite(&manifoldSuite{})

func (s *manifoldSuite) TestMachineNetworker(c *gc.C) {

	called := false

	apiCaller := basetesting.APICallerFunc(
		func(objType string,
			version int,
			id, request string,
			a, response interface{},
		) error {

			// We don't test the api call. We test that NewWorker is
			// passed the expected arguments.
			return nil
		})

	s.PatchValue(&networker.NewNetworker, func(
		st apinetworker.State,
		agentConfig agent.Config,
		intrusiveMode bool,
		configBaseDir string,
	) (worker.Worker, error) {
		called = true

		c.Assert(st, gc.NotNil)
		c.Assert(intrusiveMode, jc.IsTrue)
		xxx.Print(configBaseDir)

		// c.Assert(l, gc.FitsTypeOf, networker.DefaultListBlockDevices)
		// c.Assert(b, gc.NotNil)

		// api, ok := b.(*apinetworker.State)
		// c.Assert(ok, jc.IsTrue)
		// c.Assert(api, gc.NotNil)

		return nil, nil
	})

	a := &dummyAgent{
		tag: names.NewMachineTag("1"),
		jobs: []multiwatcher.MachineJob{
			multiwatcher.JobManageNetworking,
		},
	}

	_, err := networker.NewWorker(a, apiCaller)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(called, jc.IsTrue)
}

type dummyAgent struct {
	agent.Agent
	tag  names.Tag
	jobs []multiwatcher.MachineJob
}

func (a dummyAgent) CurrentConfig() agent.Config {
	return dummyCfg{
		tag:  a.tag,
		jobs: a.jobs,
	}
}

type dummyCfg struct {
	agent.Config
	tag  names.Tag
	jobs []multiwatcher.MachineJob
}

func (c dummyCfg) Tag() names.Tag {
	return c.tag
}
