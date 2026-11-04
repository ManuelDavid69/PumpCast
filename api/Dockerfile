# Étape 1 : compilation des binaires Go.
FROM golang:1.27-alpine AS build

WORKDIR /src

# On télécharge les dépendances dans une couche séparée pour profiter du cache Docker.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO désactivé pour obtenir un binaire statique compatible avec distroless.
ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server
RUN go build -trimpath -ldflags="-s -w" -o /out/ingest ./cmd/ingest
RUN go build -trimpath -ldflags="-s -w" -o /out/alerts ./cmd/alerts

# Étape 2 : image finale minimale, sans shell ni gestionnaire de paquets.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/server /server
COPY --from=build /out/ingest /ingest
COPY --from=build /out/alerts /alerts

EXPOSE 8080

ENTRYPOINT ["/server"]
