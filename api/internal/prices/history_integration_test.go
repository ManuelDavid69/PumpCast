package prices

import (
	"context"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/ManuelDavid69/pumpcast/api/internal/db"
)

// Test sur un vrai MongoDB, lancé seulement si MONGO_TEST_URI est défini
// (par exemple MONGO_TEST_URI=mongodb://localhost:27017 go test ./...).
// Il vérifie le comportement des time series, qu'on ne peut pas simuler.
func TestSaveDayMongo(t *testing.T) {
	uri := os.Getenv("MONGO_TEST_URI")
	if uri == "" {
		t.Skip("MONGO_TEST_URI non défini")
	}
	ctx := context.Background()

	client, err := db.Connect(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Disconnect(ctx) })

	database := client.Database("pumpcast_test_" + time.Now().Format("150405"))
	t.Cleanup(func() { _ = database.Drop(ctx) })
	if err := db.EnsureSchema(ctx, database); err != nil {
		t.Fatal(err)
	}

	repo := NewHistoryRepository(database)
	day := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
	gazole := Meta{StationID: "69100001", Fuel: "GAZOLE"}
	sp98 := Meta{StationID: "69100001", Fuel: "SP98"}

	// 1er passage : tout est nouveau
	res, err := repo.SaveDay(ctx, day, []DailyPrice{{Meta: gazole, Price: 1.629}, {Meta: sp98, Price: 1.799}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Inserted != 2 {
		t.Fatalf("1er passage : %+v", res)
	}

	// 2e passage : le gazole a changé, le SP98 non
	res, err = repo.SaveDay(ctx, day, []DailyPrice{{Meta: gazole, Price: 1.619}, {Meta: sp98, Price: 1.799}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Inserted != 0 || res.Updated != 1 || res.Unchanged != 1 {
		t.Fatalf("2e passage : %+v", res)
	}

	// Il doit rester un seul relevé gazole, avec le nouveau prix
	var docs []DailyPrice
	cur, err := database.Collection(CollectionName).Find(ctx, bson.D{{Key: "meta.fuel", Value: "GAZOLE"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := cur.All(ctx, &docs); err != nil {
		t.Fatal(err)
	}
	if len(docs) != 1 || docs[0].Price != 1.619 {
		t.Fatalf("relevés gazole = %+v, attendu un seul à 1.619", docs)
	}
}
