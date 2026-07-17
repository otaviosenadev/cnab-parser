package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Titulo representa as informações que queremos extrair de cada linha de detalhe do CNAB.
type Titulo struct {
	Sacado              string  `json:"sacado"`
	ValorOriginal       float64 `json:"valor_original"`
	DataVencimento      string  `json:"data_vencimento"`
	Pago                bool    `json:"pago"`
	DataPagamento       string  `json:"data_pagamento,omitempty"`
	ValorPago           float64 `json:"valor_pago,omitempty"`
	CodigoOcorrencia    string  `json:"codigo_ocorrencia"`
	DescricaoOcorrencia string  `json:"descricao_ocorrencia"`
}

func main() {
	porta := "8080"

	// Rota para entregar a página do dashboard (frontend)
	http.HandleFunc("/", serveIndex)

	// Rota da API que receberá o arquivo e devolverá o JSON
	http.HandleFunc("/api/parse", parseCNABHandler)

	fmt.Println("============================================================")
	fmt.Printf(" Servidor CNAB Parser iniciado com sucesso!\n")
	fmt.Printf(" Acesse no seu navegador: http://localhost:%s\n", porta)
	fmt.Println("============================================================")

	// Inicia o servidor HTTP escutando na porta configurada
	err := http.ListenAndServe(":"+porta, nil)
	if err != nil {
		fmt.Printf("Erro ao iniciar o servidor: %v\n", err)
	}
}

// serveIndex responde com o arquivo index.html que criamos no projeto
func serveIndex(w http.ResponseWriter, r *http.Request) {
	// Serve o arquivo index.html diretamente do sistema de arquivos
	http.ServeFile(w, r, "index.html")
}

// parseCNABHandler é o endpoint REST da API que processa o arquivo carregado
func parseCNABHandler(w http.ResponseWriter, r *http.Request) {
	// Apenas requisições POST são permitidas para envio de arquivos
	if r.Method != http.MethodPost {
		http.Error(w, "Método não suportado. Use POST.", http.StatusMethodNotAllowed)
		return
	}

	// Limita o tamanho do arquivo a ser recebido na memória para 10MB
	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		http.Error(w, "Arquivo enviado é muito grande (Máximo 10MB).", http.StatusBadRequest)
		return
	}

	// Obtém o arquivo enviado no formulário com o nome "file"
	arquivoUpload, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Não foi possível recuperar o arquivo da requisição: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer arquivoUpload.Close()

	var titulos []Titulo

	// Lemos o arquivo carregado linha por linha na memória (em tempo real)
	scanner := bufio.NewScanner(arquivoUpload)
	for scanner.Scan() {
		linha := scanner.Text()

		// Valida se a linha tem o tamanho mínimo esperado para conter até o Sacado (posição 274)
		if len(linha) < 274 {
			continue
		}

		// A primeira posição define o tipo de registro (0 = Header, 1 = Detalhe, 9 = Trailer)
		tipoRegistro := linha[0:1]

		if tipoRegistro == "1" {
			// Processa a linha extraindo as posições específicas do CNAB
			tituloParsed := processarLinhaDetalhe(linha)
			titulos = append(titulos, tituloParsed)
		}
	}

	if err := scanner.Err(); err != nil {
		http.Error(w, "Erro ao processar as linhas do arquivo: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Configura o cabeçalho HTTP da resposta indicando que é um JSON
	w.Header().Set("Content-Type", "application/json")
	
	// Serializa e escreve a lista de títulos convertida em JSON na resposta
	json.NewEncoder(w).Encode(titulos)
}

// processarLinhaDetalhe extrai os campos usando fatiamento de strings (slicing)
// Lembre-se: em Go os índices começam em 0 e o limite superior é exclusivo.
// Logo, a posição X a Y do manual vira linha[X-1 : Y] em Go.
func processarLinhaDetalhe(linha string) Titulo {
	// 1. Sacado (Nome do Sacado)
	// Posição no manual: 235 a 274 (Tamanho 40) -> Go: [234:274]
	sacado := strings.TrimSpace(linha[234:274])

	// 2. Valor do Título (Face)
	// Posição no manual: 127 a 139 (Tamanho 13) -> Go: [126:139]
	valorFace := converterValorMonetario(linha[126:139])

	// 3. Data de Vencimento do Título
	// Posição no manual: 121 a 126 (Tamanho 6, DDMMAA) -> Go: [120:126]
	vencimento := formatarData(linha[120:126])

	// 4. Se já foi pago (Valor pago e Data da liquidação)
	// Valor Pago: Posição 83 a 92 (Tamanho 10) -> Go [82:92]
	valorPago := converterValorMonetario(linha[82:92])

	// Data Liquidação: Posição 95 a 100 (Tamanho 6) -> Go [94:100]
	dataLiquidacaoRaw := linha[94:100]
	dataLiquidacao := formatarData(dataLiquidacaoRaw)

	foiPago := false
	if dataLiquidacao != "" && dataLiquidacaoRaw != "000000" && valorPago > 0 {
		foiPago = true
	}

	// 5. Tipo de Ocorrência
	// Posição no manual: 109 a 110 (Tamanho 2) -> Go [108:110]
	ocorrenciaRaw := linha[108:110]
	descOcorrencia := obterDescricaoOcorrencia(ocorrenciaRaw)

	return Titulo{
		Sacado:              sacado,
		ValorOriginal:       valorFace,
		DataVencimento:      vencimento,
		Pago:                foiPago,
		DataPagamento:       dataLiquidacao,
		ValorPago:           valorPago,
		CodigoOcorrencia:    ocorrenciaRaw,
		DescricaoOcorrencia: descOcorrencia,
	}
}

// converterValorMonetario converte a string do CNAB para float64 (ex: "0000000300000" = R$ 300,00)
func converterValorMonetario(valorRaw string) float64 {
	valorRaw = strings.TrimSpace(valorRaw)
	if valorRaw == "" {
		return 0.0
	}

	valorFloat, err := strconv.ParseFloat(valorRaw, 64)
	if err != nil {
		return 0.0
	}

	return valorFloat / 100.0
}

// formatarData converte a string DDMMAA do CNAB para o formato legível DD/MM/AAAA
func formatarData(dataRaw string) string {
	dataRaw = strings.TrimSpace(dataRaw)
	if dataRaw == "" || dataRaw == "000000" {
		return ""
	}

	// Em Go, a data de referência é "020106" para DDMMAA
	parsedTime, err := time.Parse("020106", dataRaw)
	if err != nil {
		return dataRaw
	}

	return parsedTime.Format("02/01/2006")
}

// obterDescricaoOcorrencia retorna a descrição amigável de cada ocorrência listada no manual
func obterDescricaoOcorrencia(codigo string) string {
	ocorrencias := map[string]string{
		"01": "Remessa",
		"04": "Abatimento",
		"06": "Alteração de vencimento",
		"11": "Aquisição de contratos futuros",
		"12": "Aquisição de baixa de contratos futuros",
		"14": "Pagamento Parcial",
		"22": "Baixa de confissão de divida",
		"68": "Baixa de Cheque",
		"71": "Baixa por recompra Paulista (com liquidação para conta administrada)",
		"72": "Recompra parcial sem adiantamento",
		"73": "Recompra parcial com adiantamento",
		"74": "Baixa por Recompra (com liquidação para o cedente)",
		"75": "Baixa por Depósito Cedente",
		"76": "Baixa por Depósito Consultoria",
		"77": "Baixa por Depósito Sacado",
		"80": "Remessa Paulista (com liquidação para conta administrada do cedente)",
		"81": "Entrada por recompra troca de títulos Paulista",
		"84": "Entrada por Recompra troca de títulos (com liquidação para o cedente)",
	}

	if desc, ok := ocorrencias[codigo]; ok {
		return desc
	}
	return "Ocorrência Não Identificada"
}
