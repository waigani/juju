// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package undertaker

import "github.com/juju/juju/api/base"

const undertakerFacade = "Undertaker"

// API provides access to the Undertaker API facade.
type API struct {
	facade base.FacadeCaller
}

// NewAPI creates a new client-side Undertaker facade.
func NewAPI(caller base.APICaller) *API {
	facadeCaller := base.NewFacadeCaller(caller, undertakerFacade)
	return &API{facade: facadeCaller}
}

// ProcessDyingEnviron calls the server-side ProcessDyingEnviron method.
func (api *API) ProcessDyingEnviron() error {
	return api.facade.FacadeCall("ProcessDyingEnviron", nil, nil)
}

// ProcessDeadEnvion calls the server-side ProcessDeadEnvion method.
func (api *API) ProcessDeadEnvion() error {
	return api.facade.FacadeCall("ProcessDeadEnvion", nil, nil)
}

// WatchUndertakes calls the server-side WatchUndertakes method.
// func (api *API) WatchUndertakes() (watcher.NotifyWatcher, error) {
// 	var result params.NotifyWatchResult
// 	err := api.facade.FacadeCall("WatchUndertakes", nil, &result)
// 	if err != nil {
// 		return nil, err
// 	}
// 	if err := result.Error; err != nil {
// 		return nil, result.Error
// 	}
// 	w := watcher.NewNotifyWatcher(api.facade.RawAPICaller(), result)
// 	return w, nil
// }
