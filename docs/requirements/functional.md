# Requisitos funcionais

Prioridade MoSCoW: **M** obrigatório, **S** devia ter, **C** podia ter.

| ID | Requisito | Prior. | Caso |
|---|---|---|---|
| RF-01 | O sistema autentica utilizadores internos por e-mail e palavra-passe. | M | UC-01 |
| RF-02 | A sessão termina com logout ou ao fim de 7 dias sem renovação; o access token dura 15 minutos. | M | UC-01 |
| RF-03 | O gestor cria, edita e desativa utilizadores e atribui o papel de gestor ou funcionário. Um utilizador desativado perde todas as sessões. | M | UC-02 |
| RF-04 | O gestor gere categorias e produtos: nome, unidade (kg, litro, unidade, molho, dúzia), preço, taxa de IVA (6, 13 ou 23%), meses de época, limiar de stock baixo e visibilidade pública. | M | UC-03 |
| RF-05 | O funcionário regista entradas de stock como lotes, com quantidade, data de colheita e validade. | M | UC-04 |
| RF-06 | O gestor regista ajustes e quebras por lote, com motivo obrigatório. | M | UC-05 |
| RF-07 | O funcionário regista vendas com várias linhas, quantidades decimais para produtos ao kg ou ao litro e método de pagamento (numerário ou MB Way). | M | UC-06 |
| RF-08 | A venda abate stock por FEFO e é recusada se algum produto não tiver stock suficiente. | M | UC-06 |
| RF-09 | Preços, IVA e totais são calculados no servidor, em cêntimos inteiros, com arredondamento por linha. | M | UC-06 |
| RF-10 | O gestor anula uma venda com motivo; o stock volta aos mesmos lotes. | S | UC-07 |
| RF-11 | O sistema mostra o stock por produto e por lote e destaca stock baixo e lotes que expiram nos próximos 3 dias. | M | UC-09 |
| RF-12 | Todos os dias às 07:00 (hora de Lisboa), o gestor recebe um e-mail com stock baixo e lotes a expirar. | S | UC-09 |
| RF-13 | O gestor consulta vendas por dia, semana e mês, os produtos mais vendidos e os totais por método de pagamento. | M | UC-08 |
| RF-14 | O gestor exporta relatórios em CSV. | C | UC-08 |
| RF-15 | O catálogo público mostra os produtos visíveis, o preço por unidade, a época e a disponibilidade (disponível, pouco, esgotado), sem mostrar quantidades exatas. | M | UC-10 |
| RF-16 | Todas as alterações de dados e inícios de sessão ficam na auditoria: quem, o quê, quando e de que IP. | M | UC-11 |
| RF-17 | O gestor pesquisa a auditoria por utilizador, entidade e período. | S | UC-11 |
| RF-18 | O ecrã de venda funciona em tablet, com toque, pesquisa por nome e teclado numérico para pesos e volumes. | S | UC-06 |
