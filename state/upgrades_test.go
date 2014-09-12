// Copyright 2014 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"time"

	"github.com/juju/errors"
	"github.com/juju/names"
	gitjujutesting "github.com/juju/testing"
	jc "github.com/juju/testing/checkers"
	"gopkg.in/mgo.v2/bson"
	"gopkg.in/mgo.v2/txn"
	gc "launchpad.net/gocheck"

	"github.com/juju/juju/constraints"
	"github.com/juju/juju/testing"
)

type upgradesSuite struct {
	testing.BaseSuite
	gitjujutesting.MgoSuite
	state *State
}

func (s *upgradesSuite) SetUpSuite(c *gc.C) {
	s.BaseSuite.SetUpSuite(c)
	s.MgoSuite.SetUpSuite(c)
}

func (s *upgradesSuite) TearDownSuite(c *gc.C) {
	s.MgoSuite.TearDownSuite(c)
	s.BaseSuite.TearDownSuite(c)
}

func (s *upgradesSuite) SetUpTest(c *gc.C) {
	s.BaseSuite.SetUpTest(c)
	s.MgoSuite.SetUpTest(c)
	var err error
	s.state, err = Initialize(TestingMongoInfo(), testing.EnvironConfig(c), TestingDialOpts(), Policy(nil))
	c.Assert(err, gc.IsNil)
}

func (s *upgradesSuite) TearDownTest(c *gc.C) {
	s.state.Close()
	s.MgoSuite.TearDownTest(c)
	s.BaseSuite.TearDownTest(c)
}

var _ = gc.Suite(&upgradesSuite{})

func (s *upgradesSuite) TestLastLoginMigrate(c *gc.C) {
	now := time.Now().UTC().Round(time.Second)
	userId := "foobar"
	oldDoc := bson.M{
		"_id_":           userId,
		"_id":            userId,
		"displayname":    "foo bar",
		"deactivated":    false,
		"passwordhash":   "hash",
		"passwordsalt":   "salt",
		"createdby":      "creator",
		"datecreated":    now,
		"lastconnection": now,
	}

	ops := []txn.Op{
		txn.Op{
			C:      "users",
			Id:     userId,
			Assert: txn.DocMissing,
			Insert: oldDoc,
		},
	}
	err := s.state.runTransaction(ops)
	c.Assert(err, gc.IsNil)

	err = MigrateUserLastConnectionToLastLogin(s.state)
	c.Assert(err, gc.IsNil)
	user, err := s.state.User(names.NewLocalUserTag(userId))
	c.Assert(err, gc.IsNil)
	c.Assert(*user.LastLogin(), gc.Equals, now)

	// check to see if _id_ field is removed
	userMap := map[string]interface{}{}
	users, closer := s.state.getCollection("users")
	defer closer()
	err = users.Find(bson.D{{"_id", userId}}).One(&userMap)
	c.Assert(err, gc.IsNil)
	_, keyExists := userMap["_id_"]
	c.Assert(keyExists, jc.IsFalse)
}

func (s *upgradesSuite) TestAddEnvUUIDToServicesID(c *gc.C) {
	serviceName := "wordpress"
	s.addServiceNoEnvID(serviceName, names.NewLocalUserTag("admin").String(), AddTestingCharm(c, s.state, serviceName), nil)

	var service serviceDoc
	services, closer := s.state.getCollection(servicesC)
	defer closer()

	err := services.Find(bson.D{{"_id", serviceName}}).One(&service)
	c.Assert(err, gc.IsNil)

	err = AddEnvUUIDToServicesID(s.state)
	c.Assert(err, gc.IsNil)

	err = services.Find(bson.D{{"_id", serviceName}}).One(&service)
	c.Assert(err, gc.ErrorMatches, "not found")

	err = services.Find(bson.D{{"_id", s.state.idForEnv(serviceName)}}).One(&service)
	c.Assert(err, gc.IsNil)
	c.Assert(service.Name, gc.Equals, serviceName)
	c.Assert(service.EnvUUID, gc.Equals, s.state.EnvironTag().Id())
}

func (s *upgradesSuite) TestAddEnvUUIDToServicesIDIdempotent(c *gc.C) {
	serviceName := "wordpress"
	s.addServiceNoEnvID(serviceName, names.NewLocalUserTag("admin").String(), AddTestingCharm(c, s.state, serviceName), nil)

	var serviceResults []serviceDoc
	services, closer := s.state.getCollection(servicesC)
	defer closer()

	err := AddEnvUUIDToServicesID(s.state)
	c.Assert(err, gc.IsNil)

	err = AddEnvUUIDToServicesID(s.state)
	c.Assert(err, gc.IsNil)

	err = services.Find(nil).All(&serviceResults)
	c.Assert(err, gc.IsNil)
	c.Assert(serviceResults, gc.HasLen, 1)

	serviceResults[0].DocID = s.state.idForEnv(serviceName)
}

// addOldService adds a service without an EnvUUID appended to ID
func (s *upgradesSuite) addServiceNoEnvID(name, owner string, ch *Charm, networks []string) (service *Service, err error) {
	st := s.state
	defer errors.Maskf(&err, "cannot add service %q", name)
	ownerTag, err := names.ParseUserTag(owner)
	if err != nil {
		return nil, errors.Annotatef(err, "Invalid ownertag %s", owner)
	}
	// Sanity checks.
	if !names.IsValidService(name) {
		return nil, errors.Errorf("invalid name")
	}
	if ch == nil {
		return nil, errors.Errorf("charm is nil")
	}
	env, err := st.Environment()
	if err != nil {
		return nil, errors.Trace(err)
	} else if env.Life() != Alive {
		return nil, errors.Errorf("environment is no longer alive")
	}
	if exists, err := isNotDead(st.db, servicesC, name); err != nil {
		return nil, errors.Trace(err)
	} else if exists {
		return nil, errors.Errorf("service already exists")
	}
	if _, err := st.EnvironmentUser(ownerTag); err != nil {
		return nil, errors.Trace(err)
	}

	type oldServiceDoc struct {
		Name string `bson:"_id"`
		serviceDoc
	}

	// Create the service addition operations.
	peers := ch.Meta().Peers
	svcDoc := &serviceDoc{
		Name:          name,
		Series:        ch.URL().Series,
		Subordinate:   ch.Meta().Subordinate,
		CharmURL:      ch.URL(),
		RelationCount: len(peers),
		Life:          Alive,
		OwnerTag:      owner,
	}
	svc := newService(st, svcDoc)
	ops := []txn.Op{
		env.assertAliveOp(),
		createConstraintsOp(st, svc.globalKey(), constraints.Value{}),
		// TODO(dimitern) 2014-04-04 bug #1302498
		// Once we can add networks independently of machine
		// provisioning, we should check the given networks are valid
		// and known before setting them.
		createRequestedNetworksOp(st, svc.globalKey(), networks),
		createSettingsOp(st, svc.settingsKey(), nil),
		{
			C:      settingsrefsC,
			Id:     svc.settingsKey(),
			Assert: txn.DocMissing,
			Insert: settingsRefsDoc{1},
		},
		{
			C:      servicesC,
			Id:     name,
			Assert: txn.DocMissing,
			Insert: svcDoc,
		}}
	// Collect peer relation addition operations.
	peerOps, err := st.addPeerRelationsOps(name, peers)
	if err != nil {
		return nil, errors.Trace(err)
	}
	ops = append(ops, peerOps...)

	if err := st.runTransaction(ops); err == txn.ErrAborted {
		err := env.Refresh()
		if (err == nil && env.Life() != Alive) || errors.IsNotFound(err) {
			return nil, errors.Errorf("environment is no longer alive")
		} else if err != nil {
			return nil, errors.Trace(err)
		}
		return nil, errors.Errorf("service already exists")
	} else if err != nil {
		return nil, errors.Trace(err)
	}
	// Refresh to pick the txn-revno.
	if err = svc.Refresh(); err != nil {
		return nil, errors.Trace(err)
	}
	return svc, nil
}

func (s *upgradesSuite) TestAddStateUsersToEnviron(c *gc.C) {
	stateBob, err := s.state.AddUser("bob", "notused", "notused", "bob")
	c.Assert(err, gc.IsNil)
	adminTag := names.NewUserTag("admin")
	bobTag := stateBob.UserTag()

	_, err = s.state.EnvironmentUser(bobTag)
	c.Assert(err, gc.ErrorMatches, `environment user "bob@local" not found`)

	err = AddStateUsersAsEnvironUsers(s.state)
	c.Assert(err, gc.IsNil)

	admin, err := s.state.EnvironmentUser(adminTag)
	c.Assert(err, gc.IsNil)
	c.Assert(admin.UserTag().Username(), gc.DeepEquals, adminTag.Username())
	bob, err := s.state.EnvironmentUser(bobTag)
	c.Assert(err, gc.IsNil)
	c.Assert(bob.UserTag().Username(), gc.DeepEquals, bobTag.Username())
}

func (s *upgradesSuite) TestAddStateUsersToEnvironIdempotent(c *gc.C) {
	stateBob, err := s.state.AddUser("bob", "notused", "notused", "bob")
	c.Assert(err, gc.IsNil)
	adminTag := names.NewUserTag("admin")
	bobTag := stateBob.UserTag()

	err = AddStateUsersAsEnvironUsers(s.state)
	c.Assert(err, gc.IsNil)

	err = AddStateUsersAsEnvironUsers(s.state)
	c.Assert(err, gc.IsNil)

	admin, err := s.state.EnvironmentUser(adminTag)
	c.Assert(admin.UserTag().Username(), gc.DeepEquals, adminTag.Username())
	bob, err := s.state.EnvironmentUser(bobTag)
	c.Assert(err, gc.IsNil)
	c.Assert(bob.UserTag().Username(), gc.DeepEquals, bobTag.Username())
}

func (s *upgradesSuite) TestAddEnvironmentUUIDToStateServerDoc(c *gc.C) {
	info, err := s.state.StateServerInfo()
	c.Assert(err, gc.IsNil)
	tag := info.EnvironmentTag

	// force remove the uuid.
	ops := []txn.Op{{
		C:      stateServersC,
		Id:     environGlobalKey,
		Assert: txn.DocExists,
		Update: bson.D{{"$unset", bson.D{
			{"env-uuid", nil},
		}}},
	}}
	err = s.state.runTransaction(ops)
	c.Assert(err, gc.IsNil)
	// Make sure it has gone.
	stateServers, closer := s.state.getCollection(stateServersC)
	defer closer()
	var doc stateServersDoc
	err = stateServers.Find(bson.D{{"_id", environGlobalKey}}).One(&doc)
	c.Assert(err, gc.IsNil)
	c.Assert(doc.EnvUUID, gc.Equals, "")

	// Run the upgrade step
	err = AddEnvironmentUUIDToStateServerDoc(s.state)
	c.Assert(err, gc.IsNil)
	// Make sure it is there now
	info, err = s.state.StateServerInfo()
	c.Assert(err, gc.IsNil)
	c.Assert(info.EnvironmentTag, gc.Equals, tag)
}

func (s *upgradesSuite) TestAddEnvironmentUUIDToStateServerDocIdempotent(c *gc.C) {
	info, err := s.state.StateServerInfo()
	c.Assert(err, gc.IsNil)
	tag := info.EnvironmentTag

	err = AddEnvironmentUUIDToStateServerDoc(s.state)
	c.Assert(err, gc.IsNil)

	info, err = s.state.StateServerInfo()
	c.Assert(err, gc.IsNil)
	c.Assert(info.EnvironmentTag, gc.Equals, tag)
}
