package domain

import "testing"

func TestListFilterNormalize(t *testing.T) {
	cases := []struct {
		name string
		in   ListFilter
		want ListFilter
	}{
		{"defaults", ListFilter{}, ListFilter{Page: 1, PerPage: 20}},
		{"respeta válidos", ListFilter{Page: 3, PerPage: 50}, ListFilter{Page: 3, PerPage: 50}},
		{"limita per_page", ListFilter{Page: 2, PerPage: 500}, ListFilter{Page: 2, PerPage: 100}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.in.Normalize(); got != tc.want {
				t.Fatalf("Normalize() = %+v, quiero %+v", got, tc.want)
			}
		})
	}
}

func TestListFilterOffset(t *testing.T) {
	if got := (ListFilter{Page: 3, PerPage: 20}).Offset(); got != 40 {
		t.Fatalf("Offset() = %d, quiero 40", got)
	}
	if got := (ListFilter{Page: 0, PerPage: 20}).Offset(); got != 0 {
		t.Fatalf("Offset() con page 0 = %d, quiero 0", got)
	}
}

func TestMemberRole(t *testing.T) {
	if !RoleOwner.CanEdit() || !RoleEditor.CanEdit() || RoleViewer.CanEdit() {
		t.Fatal("CanEdit con roles incorrectos")
	}
	if !RoleOwner.IsOwner() || RoleEditor.IsOwner() || RoleViewer.IsOwner() {
		t.Fatal("IsOwner con roles incorrectos")
	}
	if RoleOwner.IsValidRole() {
		t.Fatal("owner no debe poder asignarse como rol de miembro invitado")
	}
	if !RoleEditor.IsValidRole() || !RoleViewer.IsValidRole() {
		t.Fatal("editor y viewer deben ser roles asignables")
	}
}
