package bid_usecase

import (
	"context"
	"testing"

	"fullcycle-auction_go/internal/entity/auction_entity"
	"fullcycle-auction_go/internal/entity/bid_entity"
	"fullcycle-auction_go/internal/internal_error"

	"github.com/google/uuid"
)

type bidRepositoryDuble struct{}

func (d *bidRepositoryDuble) CreateBid(
	ctx context.Context, bidEntities []bid_entity.Bid) *internal_error.InternalError {
	return nil
}

func (d *bidRepositoryDuble) FindBidByAuctionId(
	ctx context.Context, auctionId string) ([]bid_entity.Bid, *internal_error.InternalError) {
	return nil, nil
}

func (d *bidRepositoryDuble) FindWinningBidByAuctionId(
	ctx context.Context, auctionId string) (*bid_entity.Bid, *internal_error.InternalError) {
	return nil, nil
}

type auctionRepositoryDuble struct {
	auction *auction_entity.Auction
	erro    *internal_error.InternalError
}

func (d *auctionRepositoryDuble) CreateAuction(
	ctx context.Context, auctionEntity *auction_entity.Auction) *internal_error.InternalError {
	return nil
}

func (d *auctionRepositoryDuble) FindAuctions(
	ctx context.Context, status auction_entity.AuctionStatus,
	category, productName string) ([]auction_entity.Auction, *internal_error.InternalError) {
	return nil, nil
}

func (d *auctionRepositoryDuble) FindAuctionById(
	ctx context.Context, id string) (*auction_entity.Auction, *internal_error.InternalError) {
	return d.auction, d.erro
}

func novoLanceValido() BidInputDTO {
	return BidInputDTO{
		UserId:    uuid.New().String(),
		AuctionId: uuid.New().String(),
		Amount:    1500,
	}
}

func TestCreateBid(t *testing.T) {
	t.Run("recusa lance em leilao inexistente", func(t *testing.T) {
		useCase := NewBidUseCase(&bidRepositoryDuble{}, &auctionRepositoryDuble{
			erro: internal_error.NewNotFoundError("Auction not found"),
		})

		err := useCase.CreateBid(context.Background(), novoLanceValido())
		if err == nil {
			t.Fatal("esperava erro para leilao inexistente, o lance foi aceito")
		}
		if err.Err != "not_found" {
			t.Errorf("esperava not_found (404), veio %q", err.Err)
		}
	})

	t.Run("recusa lance em leilao fechado", func(t *testing.T) {
		useCase := NewBidUseCase(&bidRepositoryDuble{}, &auctionRepositoryDuble{
			auction: &auction_entity.Auction{Id: uuid.New().String(), Status: auction_entity.Completed},
		})

		err := useCase.CreateBid(context.Background(), novoLanceValido())
		if err == nil {
			t.Fatal("esperava erro para leilao fechado, o lance foi aceito")
		}
		if err.Err != "bad_request" {
			t.Errorf("esperava bad_request (400), veio %q", err.Err)
		}
	})

	t.Run("recusa lance com id de usuario invalido", func(t *testing.T) {
		useCase := NewBidUseCase(&bidRepositoryDuble{}, &auctionRepositoryDuble{
			auction: &auction_entity.Auction{Id: uuid.New().String(), Status: auction_entity.Active},
		})

		lance := novoLanceValido()
		lance.UserId = "nao-e-uuid"

		err := useCase.CreateBid(context.Background(), lance)
		if err == nil || err.Err != "bad_request" {
			t.Errorf("esperava bad_request para user_id invalido, veio %v", err)
		}
	})

	t.Run("aceita lance em leilao aberto", func(t *testing.T) {
		useCase := NewBidUseCase(&bidRepositoryDuble{}, &auctionRepositoryDuble{
			auction: &auction_entity.Auction{Id: uuid.New().String(), Status: auction_entity.Active},
		})

		if err := useCase.CreateBid(context.Background(), novoLanceValido()); err != nil {
			t.Errorf("esperava lance aceito, veio erro %v", err.Error())
		}
	})
}
