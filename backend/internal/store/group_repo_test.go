package store

import (
	"context"
	"errors"
	"testing"
)

func TestGroupRepoCreateAndManageMembers(t *testing.T) {
	posRepo := newTestPositionRepo(t)
	db := posRepo.(*positionRepo).db
	ctx := context.Background()
	users := NewUserRepo(db)
	owner, err := users.Create(ctx, "owner@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	member, err := users.Create(ctx, "member@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	outsider, err := users.Create(ctx, "outsider@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	repo := NewGroupRepo(db)

	group, err := repo.Create(ctx, owner.ID, "Strategy Desk")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := repo.ListForUser(ctx, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ID != group.ID {
		t.Fatalf("expected owner group list, got %+v", rows)
	}
	if err := repo.AddMember(ctx, outsider.ID, group.ID, member.ID, GroupRoleMember); !errors.Is(err, ErrGroupAccessDenied) {
		t.Fatalf("expected outsider manage rejection, got %v", err)
	}
	if err := repo.AddMember(ctx, owner.ID, group.ID, member.ID, GroupRoleViewer); err != nil {
		t.Fatal(err)
	}
	rows, err = repo.ListForUser(ctx, member.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ID != group.ID {
		t.Fatalf("expected member group list, got %+v", rows)
	}
}

func TestGroupRepoAddMemberRoleProtection(t *testing.T) {
	posRepo := newTestPositionRepo(t)
	db := posRepo.(*positionRepo).db
	ctx := context.Background()
	users := NewUserRepo(db)
	mkUser := func(email string) *User {
		u, err := users.Create(ctx, email, "hash")
		if err != nil {
			t.Fatalf("create user %s: %v", email, err)
		}
		return u
	}
	owner := mkUser("owner-prot@example.com")
	admin := mkUser("admin-prot@example.com")
	member := mkUser("member-prot@example.com")
	repo := NewGroupRepo(db)

	group, err := repo.Create(ctx, owner.ID, "Protected Desk")
	if err != nil {
		t.Fatal(err)
	}
	// owner 指派 admin 為 ADMIN（正常路徑）。
	if err := repo.AddMember(ctx, owner.ID, group.ID, admin.ID, GroupRoleAdmin); err != nil {
		t.Fatalf("owner add admin: %v", err)
	}

	// ADMIN 不得自我提權為 OWNER。
	if err := repo.AddMember(ctx, admin.ID, group.ID, admin.ID, GroupRoleOwner); !errors.Is(err, ErrGroupAccessDenied) {
		t.Fatalf("expected admin self-promotion rejected, got %v", err)
	}
	// ADMIN 不得降級現任 OWNER。
	if err := repo.AddMember(ctx, admin.ID, group.ID, owner.ID, GroupRoleViewer); !errors.Is(err, ErrGroupAccessDenied) {
		t.Fatalf("expected admin demote owner rejected, got %v", err)
	}
	// ADMIN 不得把他人升為 OWNER。
	if err := repo.AddMember(ctx, admin.ID, group.ID, member.ID, GroupRoleOwner); !errors.Is(err, ErrGroupAccessDenied) {
		t.Fatalf("expected admin grant owner rejected, got %v", err)
	}
	// ADMIN 仍可管理一般成員。
	if err := repo.AddMember(ctx, admin.ID, group.ID, member.ID, GroupRoleMember); err != nil {
		t.Fatalf("expected admin add member ok, got %v", err)
	}

	// OWNER 不得修改自己的角色（避免把最後一名 OWNER 降級鎖出）。
	if err := repo.AddMember(ctx, owner.ID, group.ID, owner.ID, GroupRoleAdmin); !errors.Is(err, ErrGroupAccessDenied) {
		t.Fatalf("expected owner self-change rejected, got %v", err)
	}

	// OWNER 可指派第二位 OWNER，之後可把其中一位降級（仍留一名 OWNER）。
	if err := repo.AddMember(ctx, owner.ID, group.ID, member.ID, GroupRoleOwner); err != nil {
		t.Fatalf("owner grant second owner: %v", err)
	}
	if err := repo.AddMember(ctx, owner.ID, group.ID, member.ID, GroupRoleAdmin); err != nil {
		t.Fatalf("owner demote co-owner while another owner remains: %v", err)
	}
}
