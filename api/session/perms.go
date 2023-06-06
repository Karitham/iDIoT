package session

import (
	"fmt"
	"strings"
)

// Permission represents a permission prefix;
// it is used to check if a user has the permission to do something.
// It might be suffixed with a resource ID to check if the user has the permission to do something on a specific resource.
type Permission string

const (
	// UserCreate is the permission to create a user
	UserCreate Permission = "users:create"

	// SensorRead is the permission to read a sensor
	SensorRead Permission = "sensors:read"

	// SensorsStateUpdate is the permission to update a sensor state
	SensorsStateUpdate Permission = "sensors:state:update"
)

// Permissions is a list of permissions
type Permissions []Permission

// has checks if the list of permissions contains the given permission
func (p Permissions) has(permission ...Permission) int {
outer:
	for i, perm := range p {
		for _, p := range permission {
			if !strings.HasPrefix(string(p), string(perm)) {
				continue outer
			}
		}

		return i
	}

	return -1
}

func (p Permissions) Has(permission ...Permission) bool {
	return p.has(permission...) != -1
}

func (p Permissions) Can(permission ...Permission) error {
	h := p.has(permission...)
	if h == -1 {
		return nil
	}

	return fmt.Errorf("missing permission %s", permission[h])
}

func FromString(s ...string) Permissions {
	p := make(Permissions, len(s))
	for i, s := range s {
		p[i] = Permission(s)
	}

	return p
}

func (p Permissions) Strings() []string {
	s := make([]string, len(p))
	for i, p := range p {
		s[i] = string(p)
	}

	return s
}

func (p Permission) Customize(resourceID string) Permission {
	return Permission(string(p) + ":" + resourceID)
}
