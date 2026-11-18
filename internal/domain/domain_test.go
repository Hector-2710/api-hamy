package domain

import "testing"

func TestListFilterNormalize(t *testing.T) {
	cases := []struct {
		name string
		in   ListFilter
		want ListFilter
	}{
		{"defaults", ListFilter{}, ListFilter{Page: 1, PerPage: 20}},
		{"keeps valid values", ListFilter{Page: 3, PerPage: 50}, ListFilter{Page: 3, PerPage: 50}},
		{"caps per_page", ListFilter{Page: 2, PerPage: 500}, ListFilter{Page: 2, PerPage: 100}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.in.Normalize(); got != tc.want {
				t.Fatalf("Normalize() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestListFilterOffset(t *testing.T) {
	if got := (ListFilter{Page: 3, PerPage: 20}).Offset(); got != 40 {
		t.Fatalf("Offset() = %d, want 40", got)
	}
	if got := (ListFilter{Page: 0, PerPage: 20}).Offset(); got != 0 {
		t.Fatalf("Offset() with page 0 = %d, want 0", got)
	}
}

func TestMemberRole(t *testing.T) {
	if !RoleOwner.CanEdit() || !RoleEditor.CanEdit() || RoleViewer.CanEdit() {
		t.Fatal("CanEdit with incorrect roles")
	}
	if !RoleOwner.IsOwner() || RoleEditor.IsOwner() || RoleViewer.IsOwner() {
		t.Fatal("IsOwner with incorrect roles")
	}
	if RoleOwner.IsValidRole() {
		t.Fatal("owner must not be assignable as an invited member role")
	}
	if !RoleEditor.IsValidRole() || !RoleViewer.IsValidRole() {
		t.Fatal("editor and viewer must be assignable roles")
	}
}
