# Leilão com fechamento automático

API de leilões em Go. O projeto base (criação de leilão, lances e consultas) vem do
curso Go Expert; a feature adicionada é o encerramento automático do leilão quando sua
duração expira, sem intervenção manual.

## O que foi implementado

Tudo em `internal/infra/database/auction/create_auction.go`:

- `getAuctionInterval()` — duração do leilão a partir de `AUCTION_INTERVAL`, com padrão de 5 minutos.
- `startAuctionCloseTimer()` — goroutine que aguarda o prazo sem bloquear a resposta HTTP.
- `closeAuction()` — atualiza o status para `Completed`, filtrando por `Active` para ser idempotente.

A goroutine de fechamento usa contexto próprio: o `ctx` que chega em `CreateAuction` é
o da requisição HTTP e já foi cancelado quando o prazo vence, então o `update` morreria
com ele.

## Como rodar

```sh
docker compose up -d --build
```

API em `http://localhost:8080`, MongoDB em `localhost:27017`. Logs com
`docker compose logs -f app`, derrubar com `docker compose down -v`.

### Seed de usuários

Não existe endpoint que crie usuário, então `GET /user/:userId` só responde 404 sem
esses registros. O script é idempotente:

```sh
docker exec -i mongodb mongosh --quiet -u admin -p admin \
  --authenticationDatabase admin auctions --file /dev/stdin < seed.js
```

Semeia `8b3f6f1a-1c2d-4e5f-9a70-111111111111` (Ana Souza) e
`9c4a7e2b-2d3e-4f60-8b81-222222222222` (Bruno Lima).

### Exercitando o fechamento automático

Com `AUCTION_INTERVAL=20s`, o padrão do `.env`:

```sh
# cria o leilão
curl -X POST localhost:8080/auction \
  -H 'Content-Type: application/json' \
  -d '{"product_name":"Monitor Ultrawide","category":"Perifericos","description":"Monitor ultrawide 34 polegadas em bom estado","condition":2}'

# pega o id na listagem (o parametro status e obrigatorio; 0 lista todos)
curl 'localhost:8080/auction?status=0'

# status 0 = Active; depois de 20s o mesmo leilão volta com status 1 = Completed
curl localhost:8080/auction/<id>
```

Para incluir lances, atenção ao lote: os lances só vão para o banco quando juntam
`MAX_BATCH_SIZE` (4) ou quando passa `BATCH_INSERT_INTERVAL` (20s). Um lance isolado
perto do prazo pode ser descartado pelo próprio fechamento — mande 4 lances para
gravar na hora, ou reduza `BATCH_INSERT_INTERVAL`.

### Endpoints

| Método | Rota |
|--|--|
| POST | `/auction` — cria o leilão e agenda o fechamento |
| GET | `/auction?status=0` — lista; `status` é obrigatório, e `0` traz todos |
| GET | `/auction/:auctionId` |
| GET | `/auction/winner/:auctionId` |
| POST | `/bid` |
| GET | `/bid/:auctionId` |
| GET | `/user/:userId` |

## Variáveis de ambiente

Em `cmd/auction/.env`, carregado pela aplicação e pelo `docker-compose.yml`.

| Variável | Padrão | Para que serve |
|--|--|--|
| `AUCTION_INTERVAL` | `20s` | duração do leilão: quanto tempo após a criação ele fecha sozinho |
| `BATCH_INSERT_INTERVAL` | `20s` | intervalo máximo até gravar o lote de lances |
| `MAX_BATCH_SIZE` | `4` | quantidade de lances que fecha o lote antes do intervalo |
| `MONGODB_URL` | `mongodb://admin:admin@mongodb:27017/auctions?authSource=admin` | conexão |
| `MONGODB_DB` | `auctions` | banco da aplicação |

`AUCTION_INTERVAL` aceita qualquer duração no formato do Go (`30s`, `5m`, `1h30m`); se
faltar ou for inválida, a aplicação usa 5 minutos. Para ver o fechamento rápido:

```sh
sed -i 's/^AUCTION_INTERVAL=.*/AUCTION_INTERVAL=5s/' cmd/auction/.env
docker compose up -d --build
```

## Testes

```sh
# unitários, sem banco
go test ./internal/infra/database/auction/

# com integração (precisa de um Mongo de verdade)
docker compose up -d mongodb
MONGODB_URL_TEST='mongodb://admin:admin@localhost:27017/auctions_test?authSource=admin' \
  go test -v -count=1 ./internal/infra/database/auction/
```

Sem `MONGODB_URL_TEST`, os testes de integração são ignorados (`skip`). Eles usam o
banco `auctions_test`, separado do da aplicação, e limpam a coleção antes de cada caso.

| Teste | O que prova |
|--|--|
| `TestGetAuctionInterval` | lê a variável; cai no padrão de 5min se inválida ou ausente |
| `TestStartAuctionCloseTimer` | não bloqueia quem chamou, não dispara antes do prazo, dispara uma vez |
| `TestCloseAuctionIntegracao` | o fechamento escreve `Completed` no banco |
| `TestCreateAuctionFechaSozinhoIntegracao` | leilão nasce `Active` e fecha sozinho após o prazo |
| `TestCreateAuctionFechaMesmoComContextoDaRequisicaoCanceladoIntegracao` | o fechamento sobrevive ao fim da requisição HTTP |

Como a feature é concorrente, vale rodar com `-race` (exige `gcc`; sem ele, use o
container do Go):

```sh
docker run --rm -v "$PWD":/app -w /app \
  --network desafio-auction-goexpert_localNetwork \
  -e MONGODB_URL_TEST='mongodb://admin:admin@mongodb:27017/auctions_test?authSource=admin' \
  golang:1.24 go test -race -count=1 ./internal/infra/database/auction/
```

## Limites conhecidos

Da feature, declarado de propósito para manter o escopo do desafio:

- O agendamento vive no processo. Se a aplicação reiniciar antes do prazo, o leilão
  criado antes do restart permanece `Active` no banco — não há rotina de recuperação no
  boot. O repositório de lances já recusa lance vencido por tempo, usando a mesma
  `AUCTION_INTERVAL` sobre o `timestamp` do leilão, então um leilão vencido não aceita
  lances mesmo com o status desatualizado.
- `AUCTION_INTERVAL` é lido em dois lugares (aqui e em
  `internal/infra/database/bid/create_bid.go`, que já existia), com a mesma chave e o
  mesmo padrão. Ler chaves diferentes abriria uma janela em que o leilão aparece aberto
  e descarta lances em silêncio.

Do projeto base, medidos e deixados intactos por estarem fora do desafio:

- `GET /bid/:auctionId` sempre devolve `null`: o filtro procura `auctionId` e o
  documento gravado usa `auction_id`. Os lances existem — `GET /auction/winner/:auctionId`
  os encontra.
- `POST /bid` responde `201` mesmo para leilão fechado ou usuário inexistente; o
  descarte acontece depois, no processamento do lote, sem erro e sem log.
- `GET /auction` sem `status` responde `400`, e `?status=0` devolve todos os leilões em
  vez de só os `Active`: a listagem ignora o filtro quando o status é zero, então não há
  como pedir apenas os leilões abertos.
