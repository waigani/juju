// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package azure

import (
	"launchpad.net/gwacl"
	"launchpad.net/juju-core/constraints"
	"launchpad.net/juju-core/environs"
	"launchpad.net/juju-core/environs/config"
	"launchpad.net/juju-core/instance"
	"launchpad.net/juju-core/state"
	"launchpad.net/juju-core/state/api"
	"sync"
)

type azureEnviron struct {
	// Except where indicated otherwise, all fields in this object should
	// only be accessed using a lock or a snapshot.
	sync.Mutex

	// name is immutable; once initialized, it does not need locking.
	name string

	// ecfg is the environment's Azure-specific configuration.
	ecfg *azureEnvironConfig
}

// azureEnviron implements Environ.
var _ environs.Environ = (*azureEnviron)(nil)

// Name is specified in the Environ interface.
func (env *azureEnviron) Name() string {
	return env.name
}

// getSnapshot produces an atomic shallow copy of the environment object.
// Whenever you need to access the environment object's fields without
// modifying them, get a snapshot and read its fields instead.  You will
// get a consistent view of the fields without any further locking.
// If you do need to modify the environment's fields, do not get a snapshot
// but lock the object throughout the critical section.
func (env *azureEnviron) getSnapshot() *azureEnviron {
	env.Lock()
	defer env.Unlock()

	// Copy the environment.  (Not the pointer, the environment itself.)
	// This is a shallow copy.
	snap := *env
	// Reset the snapshot's mutex, because we just copied it while we
	// were holding it.  The snapshot will have a "clean," unlocked mutex.
	snap.Mutex = sync.Mutex{}
	return &snap
}

// Bootstrap is specified in the Environ interface.
func (env *azureEnviron) Bootstrap(cons constraints.Value) error {
	panic("unimplemented")
}

// StateInfo is specified in the Environ interface.
func (env *azureEnviron) StateInfo() (*state.Info, *api.Info, error) {
	panic("unimplemented")
}

// Config is specified in the Environ interface.
func (env *azureEnviron) Config() *config.Config {
	snap := env.getSnapshot()
	return snap.ecfg.Config
}

// SetConfig is specified in the Environ interface.
func (env *azureEnviron) SetConfig(cfg *config.Config) error {
	ecfg, err := azureEnvironProvider{}.newConfig(cfg)
	if err != nil {
		return err
	}

	env.Lock()
	defer env.Unlock()

	if env.ecfg != nil {
		_, err = azureEnvironProvider{}.Validate(cfg, env.ecfg.Config)
		if err != nil {
			return err
		}
	}

	if env.name == "" {
		// Initialization is the only time we write to the name.
		env.name = cfg.Name()
	}

	env.ecfg = ecfg
	return nil
}

// StartInstance is specified in the Environ interface.
func (env *azureEnviron) StartInstance(machineId, machineNonce string, series string, cons constraints.Value,
	info *state.Info, apiInfo *api.Info) (instance.Instance, *instance.HardwareCharacteristics, error) {
	panic("unimplemented")
}

// StopInstances is specified in the Environ interface.
func (env *azureEnviron) StopInstances([]instance.Instance) error {
	panic("unimplemented")
}

// Instances is specified in the Environ interface.
func (env *azureEnviron) Instances(ids []instance.Id) ([]instance.Instance, error) {
	panic("unimplemented")
}

// AllInstances is specified in the Environ interface.
func (env *azureEnviron) AllInstances() ([]instance.Instance, error) {
	panic("unimplemented")
}

// Storage is specified in the Environ interface.
func (env *azureEnviron) Storage() environs.Storage {
	context := &environStorageContext{environ: env}
	return &azureStorage{context}
}

// PublicStorage is specified in the Environ interface.
func (env *azureEnviron) PublicStorage() environs.StorageReader {
	context := &publicEnvironStorageContext{environ: env}
	if context.getContainer() != "" {
		return &azureStorage{context}
	}
	return environs.EmptyStorage
}

// Destroy is specified in the Environ interface.
func (env *azureEnviron) Destroy(insts []instance.Instance) error {
	panic("unimplemented")
}

// OpenPorts is specified in the Environ interface.
func (env *azureEnviron) OpenPorts(ports []instance.Port) error {
	panic("unimplemented")
}

// ClosePorts is specified in the Environ interface.
func (env *azureEnviron) ClosePorts(ports []instance.Port) error {
	panic("unimplemented")
}

// Ports is specified in the Environ interface.
func (env *azureEnviron) Ports() ([]instance.Port, error) {
	panic("unimplemented")
}

// Provider is specified in the Environ interface.
func (env *azureEnviron) Provider() environs.EnvironProvider {
	panic("unimplemented")
}

// azureManagementContext wraps two things: a gwacl.ManagementAPI (effectively
// a session on the Azure management API) and a tempCertFile, which keeps track
// of the temporary certificate file that needs to be deleted once we're done
// with this particular session.
// Since it embeds *gwacl.ManagementAPI, you can use it much as if it were a
// pointer to a ManagementAPI object.  Just don't forget to release it after
// use.
type azureManagementContext struct {
	*gwacl.ManagementAPI
	certFile *tempCertFile
}

// getManagementAPI obtains a context object for interfacing with Azure's
// management API.
// For now, each invocation just returns a separate object.  This is probably
// wasteful (each context gets its own SSL connection) and may need optimizing
// later.
func (env *azureEnviron) getManagementAPI() (*azureManagementContext, error) {
	snap := env.getSnapshot()
	subscription := snap.ecfg.ManagementSubscriptionId()
	certData := snap.ecfg.ManagementCertificate()
	certFile, err := newTempCertFile([]byte(certData))
	if err != nil {
		return nil, err
	}
	// After this point, if we need to leave prematurely, we should clean
	// up that certificate file.
	mgtAPI, err := gwacl.NewManagementAPI(subscription, certFile.Path())
	if err != nil {
		certFile.Delete()
		return nil, err
	}
	context := azureManagementContext{
		ManagementAPI: mgtAPI,
		certFile:      certFile,
	}
	return &context, nil
}

// releaseManagementAPI frees up a context object obtained through
// getManagementAPI.
func (env *azureEnviron) releaseManagementAPI(context *azureManagementContext) {
	// Be tolerant to incomplete context objects, in case we ever get
	// called during cleanup of a failed attempt to create one.
	if context == nil || context.certFile == nil {
		return
	}
	// For now, all that needs doing is to delete the temporary certificate
	// file.  We may do cleverer things later, such as connection pooling
	// where this method returns a context to the pool.
	context.certFile.Delete()
}

// getStorageContext obtains a context object for interfacing with Azure's
// storage API.
// For now, each invocation just returns a separate object.  This is probably
// wasteful (each context gets its own SSL connection) and may need optimizing
// later.
func (env *azureEnviron) getStorageContext() (*gwacl.StorageContext, error) {
	snap := env.getSnapshot()
	context := gwacl.StorageContext{
		Account: snap.ecfg.StorageAccountName(),
		Key:     snap.ecfg.StorageAccountKey(),
	}
	// There is currently no way for this to fail.
	return &context, nil
}

// getPublicStorageContext obtains a context object for interfacing with
// Azure's storage API (public storage).
func (env *azureEnviron) getPublicStorageContext() (*gwacl.StorageContext, error) {
	snap := env.getSnapshot()
	context := gwacl.StorageContext{
		Account: snap.ecfg.PublicStorageAccountName(),
		Key:     "", // Empty string means anonymous access.
	}
	// There is currently no way for this to fail.
	return &context, nil
}
