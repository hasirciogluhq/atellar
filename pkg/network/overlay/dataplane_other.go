//go:build !linux

package overlay

import (
	"errors"
)

var errUnsupported = errors.New("overlay networking requires linux")

// NewDataplane returns an implementation that fails every call: the overlay
// needs the Linux VXLAN driver.
func NewDataplane() Dataplane { return unsupported{} }

type unsupported struct{}

func (unsupported) EnsureBridge(Bridge) error                { return errUnsupported }
func (unsupported) EnsureVXLAN(VXLAN) error                  { return errUnsupported }
func (unsupported) ListRoutes(string) ([]Route, error)       { return nil, errUnsupported }
func (unsupported) ReplaceRoute(string, Route) error         { return errUnsupported }
func (unsupported) DeleteRoute(string, Route) error          { return errUnsupported }
func (unsupported) ListNeighbors(string) ([]Neighbor, error) { return nil, errUnsupported }
func (unsupported) ReplaceNeighbor(string, Neighbor) error   { return errUnsupported }
func (unsupported) DeleteNeighbor(string, Neighbor) error    { return errUnsupported }
func (unsupported) ListFDB(string) ([]FDBEntry, error)       { return nil, errUnsupported }
func (unsupported) ReplaceFDB(string, FDBEntry) error        { return errUnsupported }
func (unsupported) DeleteFDB(string, FDBEntry) error         { return errUnsupported }
