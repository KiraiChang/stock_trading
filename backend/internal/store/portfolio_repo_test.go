package store

import (
	"context"
	"testing"
)

func TestPortfolioRepoListsLegacyAndCreatesUserPortfolio(t *testing.T) {
	posRepo := newTestPositionRepo(t)
	db := posRepo.(*positionRepo).db
	ctx := context.Background()
	user, err := NewUserRepo(db).Create(ctx, "portfolio@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	repo := NewPortfolioRepo(db)

	rows, err := repo.ListForUser(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ID != DefaultPortfolioID || rows[0].OwnerType != PortfolioOwnerTenant {
		t.Fatalf("expected legacy default portfolio, got %+v", rows)
	}

	created, err := repo.CreateForUser(ctx, user.ID, "Swing Trades")
	if err != nil {
		t.Fatal(err)
	}
	if created.OwnerType != PortfolioOwnerUser || !created.OwnerID.Valid || uint64(created.OwnerID.Int64) != user.ID {
		t.Fatalf("unexpected created portfolio: %+v", created)
	}
	rows, err = repo.ListForUser(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected legacy + user portfolio, got %+v", rows)
	}
}

func TestPortfolioRepoGroupPortfolioAccess(t *testing.T) {
	posRepo := newTestPositionRepo(t)
	db := posRepo.(*positionRepo).db
	ctx := context.Background()
	users := NewUserRepo(db)
	owner, err := users.Create(ctx, "group-owner@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	viewer, err := users.Create(ctx, "group-viewer@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	outsider, err := users.Create(ctx, "group-outsider@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	groups := NewGroupRepo(db)
	group, err := groups.Create(ctx, owner.ID, "Desk A")
	if err != nil {
		t.Fatal(err)
	}
	if err := groups.AddMember(ctx, owner.ID, group.ID, viewer.ID, GroupRoleViewer); err != nil {
		t.Fatal(err)
	}
	portfolios := NewPortfolioRepo(db)
	groupPortfolio, err := portfolios.CreateForGroup(ctx, owner.ID, group.ID, "Desk A Portfolio")
	if err != nil {
		t.Fatal(err)
	}

	canRead, err := portfolios.CanAccess(ctx, viewer.ID, groupPortfolio.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	canWrite, err := portfolios.CanAccess(ctx, viewer.ID, groupPortfolio.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	outsiderRead, err := portfolios.CanAccess(ctx, outsider.ID, groupPortfolio.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if !canRead || canWrite || outsiderRead {
		t.Fatalf("unexpected access states: viewer read=%v write=%v outsider read=%v", canRead, canWrite, outsiderRead)
	}

	if err := groups.AddMember(ctx, owner.ID, group.ID, viewer.ID, GroupRoleAdmin); err != nil {
		t.Fatal(err)
	}
	canWrite, err = portfolios.CanAccess(ctx, viewer.ID, groupPortfolio.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if !canWrite {
		t.Fatal("expected group admin to have write access")
	}
}
