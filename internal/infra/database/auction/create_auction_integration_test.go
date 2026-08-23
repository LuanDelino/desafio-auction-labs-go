package auction

import (
	"context"
	"os"
	"testing"
	"time"

	"fullcycle-auction_go/internal/entity/auction_entity"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Roda apenas quando MONGODB_URL_TEST aponta para um Mongo de verdade.
func novoRepositorioDeTeste(t *testing.T) *AuctionRepository {
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

	database := client.Database("auctions_test")
	repository := NewAuctionRepository(database)

	if _, err := repository.Collection.DeleteMany(ctx, map[string]any{}); err != nil {
		t.Fatalf("erro limpando a colecao de teste: %v", err)
	}

	t.Cleanup(func() {
		desconectar, cancelar := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelar()
		_ = client.Disconnect(desconectar)
	})

	return repository
}

func criarLeilaoDeTeste(t *testing.T) *auction_entity.Auction {
	t.Helper()

	auctionEntity, err := auction_entity.CreateAuction(
		"Monitor Ultrawide",
		"Perifericos",
		"Monitor ultrawide de 34 polegadas em bom estado",
		auction_entity.Used)
	if err != nil {
		t.Fatalf("erro montando o leilao de teste: %v", err.Error())
	}

	return auctionEntity
}

func TestCloseAuctionIntegracao(t *testing.T) {
	repository := novoRepositorioDeTeste(t)
	ctx := context.Background()

	auctionEntity := criarLeilaoDeTeste(t)
	if err := repository.CreateAuction(ctx, auctionEntity); err != nil {
		t.Fatalf("erro inserindo o leilao: %v", err.Error())
	}

	if err := repository.closeAuction(ctx, auctionEntity.Id); err != nil {
		t.Fatalf("erro fechando o leilao: %v", err.Error())
	}

	fechado, err := repository.FindAuctionById(ctx, auctionEntity.Id)
	if err != nil {
		t.Fatalf("erro buscando o leilao fechado: %v", err.Error())
	}
	if fechado.Status != auction_entity.Completed {
		t.Errorf("esperava status Completed, veio %d", fechado.Status)
	}
}

func TestCreateAuctionFechaSozinhoIntegracao(t *testing.T) {
	t.Setenv("AUCTION_INTERVAL", "1s")

	repository := novoRepositorioDeTeste(t)
	ctx := context.Background()

	auctionEntity := criarLeilaoDeTeste(t)
	if err := repository.CreateAuction(ctx, auctionEntity); err != nil {
		t.Fatalf("erro criando o leilao: %v", err.Error())
	}

	recemCriado, err := repository.FindAuctionById(ctx, auctionEntity.Id)
	if err != nil {
		t.Fatalf("erro buscando o leilao recem criado: %v", err.Error())
	}
	if recemCriado.Status != auction_entity.Active {
		t.Fatalf("leilao deveria nascer Active, veio %d", recemCriado.Status)
	}

	time.Sleep(2 * time.Second)

	depois, err := repository.FindAuctionById(ctx, auctionEntity.Id)
	if err != nil {
		t.Fatalf("erro buscando o leilao depois do prazo: %v", err.Error())
	}
	if depois.Status != auction_entity.Completed {
		t.Errorf("leilao deveria ter fechado sozinho apos AUCTION_INTERVAL, status veio %d", depois.Status)
	}
}

// O ctx da requisicao morre antes do prazo; o fechamento nao pode depender dele.
func TestCreateAuctionFechaMesmoComContextoDaRequisicaoCanceladoIntegracao(t *testing.T) {
	t.Setenv("AUCTION_INTERVAL", "1s")

	repository := novoRepositorioDeTeste(t)

	ctxRequisicao, cancelarRequisicao := context.WithCancel(context.Background())

	auctionEntity := criarLeilaoDeTeste(t)
	if err := repository.CreateAuction(ctxRequisicao, auctionEntity); err != nil {
		t.Fatalf("erro criando o leilao: %v", err.Error())
	}

	cancelarRequisicao()

	time.Sleep(2 * time.Second)

	depois, err := repository.FindAuctionById(context.Background(), auctionEntity.Id)
	if err != nil {
		t.Fatalf("erro buscando o leilao depois do prazo: %v", err.Error())
	}
	if depois.Status != auction_entity.Completed {
		t.Errorf("leilao deveria fechar mesmo com o contexto da requisicao cancelado, status veio %d", depois.Status)
	}
}
