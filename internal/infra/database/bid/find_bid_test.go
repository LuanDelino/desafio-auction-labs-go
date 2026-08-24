package bid

import (
	"context"
	"os"
	"testing"
	"time"

	"fullcycle-auction_go/internal/entity/auction_entity"
	"fullcycle-auction_go/internal/entity/bid_entity"
	"fullcycle-auction_go/internal/infra/database/auction"

	"github.com/google/uuid"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Roda apenas quando MONGODB_URL_TEST aponta para um Mongo de verdade.
func novoRepositorioDeTeste(t *testing.T) (*BidRepository, *auction.AuctionRepository) {
	t.Helper()

	mongoURL := os.Getenv("MONGODB_URL_TEST")
	if mongoURL == "" {
		t.Skip("MONGODB_URL_TEST nao definida: teste de integracao ignorado")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURL))
	if err != nil {
		t.Fatalf("erro conectando no mongo de teste: %v", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		t.Fatalf("mongo de teste nao respondeu ao ping: %v", err)
	}

	database := client.Database("auctions_test_bid")
	auctionRepository := auction.NewAuctionRepository(database)
	bidRepository := NewBidRepository(database, auctionRepository)

	for _, collection := range []*mongo.Collection{bidRepository.Collection, auctionRepository.Collection} {
		if _, err := collection.DeleteMany(ctx, map[string]any{}); err != nil {
			t.Fatalf("erro limpando a colecao de teste: %v", err)
		}
	}

	t.Cleanup(func() {
		desconectar, cancelar := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelar()
		_ = client.Disconnect(desconectar)
	})

	return bidRepository, auctionRepository
}

func TestFindBidByAuctionIdIntegracao(t *testing.T) {
	t.Setenv("AUCTION_INTERVAL", "1m")

	bidRepository, auctionRepository := novoRepositorioDeTeste(t)
	ctx := context.Background()

	auctionEntity, errEntity := auction_entity.CreateAuction(
		"Monitor Ultrawide", "Perifericos",
		"Monitor ultrawide de 34 polegadas em bom estado", auction_entity.Used)
	if errEntity != nil {
		t.Fatalf("erro montando o leilao: %v", errEntity.Error())
	}
	if err := auctionRepository.CreateAuction(ctx, auctionEntity); err != nil {
		t.Fatalf("erro inserindo o leilao: %v", err.Error())
	}

	primeiro, _ := bid_entity.CreateBid(novoUUID(), auctionEntity.Id, 1200)
	segundo, _ := bid_entity.CreateBid(novoUUID(), auctionEntity.Id, 1800)
	if err := bidRepository.CreateBid(ctx, []bid_entity.Bid{*primeiro, *segundo}); err != nil {
		t.Fatalf("erro inserindo os lances: %v", err.Error())
	}

	lances, err := bidRepository.FindBidByAuctionId(ctx, auctionEntity.Id)
	if err != nil {
		t.Fatalf("erro buscando os lances: %v", err.Error())
	}
	if len(lances) != 2 {
		t.Fatalf("esperava 2 lances do leilao, veio %d", len(lances))
	}
}

func novoUUID() string {
	return uuid.New().String()
}
