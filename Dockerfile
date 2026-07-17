# ===================================================
# ESTÁGIO 1: Compilação do binário Go
# ===================================================
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copia os arquivos de configuração do módulo
COPY go.mod ./

# Copia o código fonte do backend
COPY main.go ./

# Compila o binário otimizado de forma estática (sem dependências externas de C)
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o cnab-parser .

# ===================================================
# ESTÁGIO 2: Imagem final de execução super leve (Alpine)
# ===================================================
FROM alpine:3.19

WORKDIR /app

# Instala certificados SSL e ferramentas básicas de segurança
RUN apk --no-cache add ca-certificates tzdata

# Copia apenas o executável final gerado no estágio anterior
COPY --from=builder /app/cnab-parser .

# Copia o arquivo da interface web (frontend)
COPY index.html .

# Exponha a porta do servidor
EXPOSE 8080

# Define o fuso horário para o Brasil (opcional, bom para logs corretos)
ENV TZ=America/Sao_Paulo

# Comando que inicia o nosso servidor
CMD ["./cnab-parser"]
