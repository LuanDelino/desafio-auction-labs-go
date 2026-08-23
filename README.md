# Leilão com fechamento automático

API de leilões em Go com fechamento automático por tempo. O projeto base (criação de
leilão, lances e consultas) vem do curso Go Expert; a feature adicionada aqui é o
encerramento automático do leilão quando sua duração expira, sem intervenção manual.

## O que foi implementado

Ao criar um leilão, o repositório dispara uma goroutine que aguarda a duração
configurada e então atualiza o status do leilão para `Completed` (fechado) no MongoDB.

Toda a implementação está em `internal/infra/database/auction/create_auction.go`:

- `getAuctionInterval()` — lê a duração do leilão da variável de ambiente `AUCTION_INTERVAL`.
- `startAuctionCloseTimer()` — agenda a ação de fechamento em segundo plano, sem
  bloquear a rotina que criou o leilão.
- `closeAuction()` — atualiza o status no banco. O filtro exige status `Active`, então
  fechar duas vezes não sobrescreve nada.

Dois pontos de atenção que a implementação trata:

1. O `context.Context` que chega em `CreateAuction` é o da requisição HTTP e já foi
   cancelado quando o prazo vence. A goroutine de fechamento usa contexto próprio,
   senão o `update` morreria junto com a requisição.
2. O agendamento não bloqueia a resposta HTTP: a criação do leilão devolve `201`
   imediatamente e o fechamento acontece depois, em paralelo.

## Como rodar

Requisitos: Docker e Docker Compose.

```sh
docker compose up -d --build
```

A API sobe em `http://localhost:8080` e o MongoDB em `localhost:27017`.

Para acompanhar os logs e derrubar o ambiente:

```sh
docker compose logs -f app
docker compose down -v
```

### Exercitando o fechamento automático

Com `AUCTION_INTERVAL=20s` (valor padrão do `.env`):

```sh
# cria o leilão
curl -X POST localhost:8080/auction \
  -H 'Content-Type: application/json' \
  -d '{"product_name":"Teclado Mecanico","category":"Perifericos","description":"Teclado mecanico switch marrom, pouco uso","condition":1}'

# consulta o leilão (status 0 = Active)
curl localhost:8080/auction

# depois de 20 segundos, o mesmo leilão volta com status 1 = Completed
curl localhost:8080/auction/<id-do-leilao>
```

### Endpoints

| Método | Rota | Descrição |
|--|--|--|
| POST | `/auction` | cria um leilão (e agenda o fechamento) |
| GET | `/auction` | lista leilões, filtros `status`, `category`, `productName` |
| GET | `/auction/:auctionId` | busca leilão por id |
| GET | `/auction/winner/:auctionId` | lance vencedor do leilão |
| POST | `/bid` | cria um lance |
| GET | `/bid/:auctionId` | lances de um leilão |
| GET | `/user/:userId` | busca usuário por id |

## Variáveis de ambiente

Ficam em `cmd/auction/.env`, carregado pela aplicação e pelo `docker-compose.yml`.

| Variável | Padrão | Para que serve |
|--|--|--|
| `AUCTION_INTERVAL` | `20s` | duração do leilão: quanto tempo depois da criação ele fecha sozinho |
| `BATCH_INSERT_INTERVAL` | `20s` | intervalo máximo até a gravação do lote de lances |
| `MAX_BATCH_SIZE` | `4` | quantidade de lances que fecha um lote antes do intervalo |
| `MONGODB_URL` | `mongodb://admin:admin@mongodb:27017/auctions?authSource=admin` | conexão com o MongoDB |
| `MONGODB_DB` | `auctions` | banco usado pela aplicação |

`AUCTION_INTERVAL` aceita qualquer duração no formato do Go (`30s`, `5m`, `1h30m`).
Se estiver ausente ou inválida, a aplicação usa 5 minutos.

Para ver o fechamento acontecer rápido, reduza o valor e suba o ambiente de novo:

```sh
sed -i 's/^AUCTION_INTERVAL=.*/AUCTION_INTERVAL=5s/' cmd/auction/.env
docker compose up -d --build
```

## Testes

Os testes ficam em `internal/infra/database/auction/`.

### Unitários (não precisam de banco)

```sh
go test ./internal/infra/database/auction/
```

Cobrem a leitura de `AUCTION_INTERVAL` (valor válido, valor inválido e ausência, caindo
no padrão de 5 minutos) e o agendamento em segundo plano (não bloqueia quem chamou,
não dispara antes do prazo, dispara uma única vez depois dele).

### Integração (precisam de um MongoDB de verdade)

Provam o cenário do desafio de ponta a ponta: leilão criado como aberto, e status
alterado para fechado sozinho depois de `AUCTION_INTERVAL`.

```sh
docker compose up -d mongodb

MONGODB_URL_TEST='mongodb://admin:admin@localhost:27017/auctions_test?authSource=admin' \
  go test -v -count=1 ./internal/infra/database/auction/
```

Sem `MONGODB_URL_TEST` definida, esses testes são ignorados (`skip`) e apenas os
unitários rodam. Eles usam o banco `auctions_test`, separado do banco da aplicação, e
limpam a coleção antes de cada caso.

Três casos de integração:

| Teste | O que prova |
|--|--|
| `TestCloseAuctionIntegracao` | o fechamento escreve `Completed` no banco |
| `TestCreateAuctionFechaSozinhoIntegracao` | leilão nasce `Active` e fecha sozinho após o prazo |
| `TestCreateAuctionFechaMesmoComContextoDaRequisicaoCanceladoIntegracao` | o fechamento sobrevive ao fim da requisição HTTP que criou o leilão |

### Detector de corrida

A feature é concorrente, então vale rodar com `-race` (exige `gcc`; se a máquina não
tiver, use o container do Go):

```sh
docker run --rm -v "$PWD":/app -w /app \
  --network desafio-auction-goexpert_localNetwork \
  -e MONGODB_URL_TEST='mongodb://admin:admin@mongodb:27017/auctions_test?authSource=admin' \
  golang:1.24 go test -race -count=1 ./internal/infra/database/auction/
```

## Limites conhecidos

Declarados de propósito, para manter o escopo no que o desafio pediu:

- O agendamento vive no processo. Se a aplicação reiniciar antes do prazo, o leilão
  criado antes do restart permanece `Active` no banco — não existe rotina de
  recuperação no boot. Vale notar que o repositório de lances já recusa lance por
  tempo (`AUCTION_INTERVAL` sobre o `timestamp` do leilão), então um leilão vencido não
  aceita lances mesmo que o status no banco não tenha sido atualizado.
- `AUCTION_INTERVAL` é lido em dois lugares (aqui e em
  `internal/infra/database/bid/create_bid.go`, que já existia). A chave e o padrão são
  os mesmos nos dois, e o código pré-existente foi deixado intacto.
