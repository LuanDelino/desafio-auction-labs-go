package auction

import (
	"context"
	"testing"

	"fullcycle-auction_go/internal/entity/auction_entity"
)

func TestFindAuctionsPorStatusIntegracao(t *testing.T) {
	repository := novoRepositorioDeTeste(t)
	ctx := context.Background()

	aberto := criarLeilaoDeTeste(t)
	fechado := criarLeilaoDeTeste(t)

	for _, auctionEntity := range []*auction_entity.Auction{aberto, fechado} {
		if err := repository.CreateAuction(ctx, auctionEntity); err != nil {
			t.Fatalf("erro inserindo o leilao: %v", err.Error())
		}
	}
	if err := repository.closeAuction(ctx, fechado.Id); err != nil {
		t.Fatalf("erro fechando o leilao: %v", err.Error())
	}

	t.Run("filtra apenas os leiloes abertos", func(t *testing.T) {
		encontrados, err := repository.FindAuctions(ctx, auction_entity.Active, "", "")
		if err != nil {
			t.Fatalf("erro listando leiloes abertos: %v", err.Error())
		}
		if len(encontrados) != 1 {
			t.Fatalf("esperava 1 leilao aberto, veio %d", len(encontrados))
		}
		if encontrados[0].Id != aberto.Id {
			t.Errorf("veio o leilao errado: %s", encontrados[0].Id)
		}
	})

	t.Run("filtra apenas os leiloes fechados", func(t *testing.T) {
		encontrados, err := repository.FindAuctions(ctx, auction_entity.Completed, "", "")
		if err != nil {
			t.Fatalf("erro listando leiloes fechados: %v", err.Error())
		}
		if len(encontrados) != 1 || encontrados[0].Id != fechado.Id {
			t.Fatalf("esperava so o leilao fechado, veio %d resultado(s)", len(encontrados))
		}
	})

	t.Run("sem filtro de status devolve todos", func(t *testing.T) {
		encontrados, err := repository.FindAuctions(ctx, auction_entity.NoStatusFilter, "", "")
		if err != nil {
			t.Fatalf("erro listando todos os leiloes: %v", err.Error())
		}
		if len(encontrados) != 2 {
			t.Errorf("esperava 2 leiloes, veio %d", len(encontrados))
		}
	})
}

func TestFindAuctionByIdInexistenteIntegracao(t *testing.T) {
	repository := novoRepositorioDeTeste(t)

	_, err := repository.FindAuctionById(context.Background(), "68e5d7a2-1d3f-4c7b-9a11-000000000000")
	if err == nil {
		t.Fatal("esperava erro para leilao inexistente")
	}
	if err.Err != "not_found" {
		t.Errorf("esperava not_found (404), veio %q", err.Err)
	}
}
