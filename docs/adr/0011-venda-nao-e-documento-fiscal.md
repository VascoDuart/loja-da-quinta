# 0011. A venda ao balcão não é documento fiscal

- **Estado:** Aceite
- **Data:** 2026-11-18

## Contexto

Em Portugal, emitir faturas exige software certificado pela Autoridade Tributária (SAF-T, assinatura e numeração sequencial). Certificar software é um processo longo e fora do âmbito de um projeto de portefólio.

## Decisão

A venda ao balcão é um **registo interno** de vendas e stock. A fatura continua a ser emitida no software certificado que a loja já usa.

## Consequências

- O sistema não emite nem imprime nada que possa ser confundido com uma fatura.
- A interface diz claramente “registo de venda”.
- Uma integração futura com software de faturação certificado exige um ADR próprio.
