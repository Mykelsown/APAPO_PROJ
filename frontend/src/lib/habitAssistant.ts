import { formatKobo } from './money'
import type { SpendingStats } from './spendingStats'

/** Parses amount in Naira (e.g. "50k", "₦50,000", "50000", "2.5m") into kobo */
function extractAmountKobo(text: string): number | null {
  const match = text.match(/(?:₦|n|ngn)?\s*(\d+(?:,\d{3})*(?:\.\d+)?)\s*(k|m|million|thousand|kobo|naira)?\b/i)
  if (!match) return null
  const rawNum = parseFloat(match[1].replace(/,/g, ''))
  if (isNaN(rawNum) || rawNum <= 0) return null
  const unit = (match[2] || '').toLowerCase()
  if (unit === 'k' || unit === 'thousand') return Math.round(rawNum * 1_000 * 100)
  if (unit === 'm' || unit === 'million') return Math.round(rawNum * 1_000_000 * 100)
  if (unit === 'kobo') return Math.round(rawNum)
  return Math.round(rawNum * 100)
}

/** Rule-based answers for the spending assistant, computed only from the user's own numbers. */
export function answerHabitQuestion(rawQuestion: string, stats: SpendingStats, accountCount: number): string {
  const q = rawQuestion.toLowerCase().trim()
  const {
    bankComparisonStats,
    categoryStats,
    merchantStats,
    recurringCharges,
    weekendVsWeekday,
    totalBalanceKobo,
    dailyBurnKobo,
    runwayDays,
    totalOutflowKobo,
    totalInflowKobo,
    netCashflowKobo,
    savingsRate,
    highestExpense,
  } = stats

  // 1. Affordability check (e.g. "can I afford 50k?", "can I spend 20,000?", "can I buy")
  if (q.includes('afford') || q.includes('can i spend') || q.includes('can i pay') || q.includes('can i buy') || q.includes('enough for')) {
    const targetKobo = extractAmountKobo(q)
    if (targetKobo !== null) {
      if (targetKobo > totalBalanceKobo) {
        const deficit = targetKobo - totalBalanceKobo
        return `You cannot currently afford ${formatKobo(targetKobo)}. Your combined liquid balance is ${formatKobo(totalBalanceKobo)}, leaving a deficit of ${formatKobo(deficit)} across your ${accountCount || 4} linked accounts.`
      }
      const remainder = totalBalanceKobo - targetKobo
      const remainingDays = dailyBurnKobo > 0 ? Math.floor(remainder / dailyBurnKobo) : 999
      return `Yes, you can afford ${formatKobo(targetKobo)}. After this payment, you will have ${formatKobo(remainder)} remaining across accounts (~${remainingDays} days of normal burn).`
    }
  }

  // 2. Highest single expense / biggest payment
  if (
    q.includes('highest expense') ||
    q.includes('biggest expense') ||
    q.includes('largest expense') ||
    q.includes('most expensive') ||
    q.includes('top expense') ||
    q.includes('highest spend') ||
    q.includes('biggest spend') ||
    q.includes('largest spend') ||
    q.includes('biggest payment') ||
    q.includes('highest payment')
  ) {
    if (highestExpense) {
      return `Your highest single expense was ${formatKobo(highestExpense.amountKobo)} to ${highestExpense.counterparty} (${highestExpense.category}) via ${highestExpense.bankName}.`
    }
    if (merchantStats && merchantStats.length > 0) {
      const topM = merchantStats[0]
      return `Your highest spending destination is ${topM.merchant} at ${formatKobo(topM.totalKobo)} across ${topM.count} transactions (${topM.percent}% of total outflow).`
    }
    return 'No outgoing debit transactions recorded in this period.'
  }

  // 3. Subscriptions and recurring bills
  if (q.includes('subscription') || q.includes('recurring') || q.includes('bills?') || q.includes('recurring bill')) {
    if (recurringCharges.length === 0) {
      return 'No recurring subscriptions detected. Regular fixed payments like electricity (IKEDC), streaming (Netflix, Spotify), or telco data will appear here once debited.'
    }
    const listStr = recurringCharges.map((r) => `${r.name}: ${formatKobo(r.amountKobo)}`).join(', ')
    return `Identified ${recurringCharges.length} recurring commitments: ${listStr}. Total recurring monthly overhead is approximately ${formatKobo(recurringCharges.reduce((s, r) => s + r.amountKobo, 0))}.`
  }

  // 4. Top merchant / payee leaderboard
  if (q.includes('merchant') || q.includes('who did i pay') || q.includes('recipient') || q.includes('payee') || q.includes('vendor')) {
    if (!merchantStats || merchantStats.length === 0) return 'No merchant debit records found.'
    const topM = merchantStats[0]
    const list = merchantStats.slice(0, 3).map((m) => `${m.merchant} (${formatKobo(m.totalKobo)}, ${m.count} txns)`).join('; ')
    return `Your top recipient is ${topM.merchant} with ${formatKobo(topM.totalKobo)} in debits (${topM.percent}% of outflow). Top destinations: ${list}.`
  }

  // 5. Bank drainage / fastest draining
  if (q.includes('draining') || q.includes('fastest') || q.includes('bleeding') || (q.includes('bank') && (q.includes('drain') || q.includes('drop')))) {
    if (bankComparisonStats.length === 0) return 'No debit transactions recorded yet across your linked accounts.'
    const topBank = bankComparisonStats[0]
    return `Your fastest draining account is ${topBank.bankName}, accounting for ${formatKobo(topBank.outflowKobo)} across ${topBank.count} transactions. We recommend balancing outbound payments via Split-Pay to keep your reserves intact.`
  }

  // 6. Split-Pay explanation / questions
  if (q.includes('split-pay') || q.includes('split pay') || (q.includes('how') && q.includes('split'))) {
    return 'Split-Pay debits multiple linked bank accounts in a single tap when one balance is insufficient, using an atomic two-phase commit with automatic rollback if any leg fails.'
  }

  // 7. Specific bank query
  const matchedBank = bankComparisonStats.find((b) => q.includes(b.bankName.toLowerCase()))
  if (matchedBank && (q.includes('how much') || q.includes('spend') || q.includes('flow') || q.includes('balance') || q.includes('activity'))) {
    return `${matchedBank.bankName} has recorded ${formatKobo(matchedBank.outflowKobo)} in outflows and ${formatKobo(matchedBank.inflowKobo)} in inflows across ${matchedBank.count} transactions (net: ${formatKobo(matchedBank.netKobo)}).`
  }

  // 8. Specific category queries with dynamic matching
  const matchedCat =
    categoryStats.find((c) => q.includes(c.category.toLowerCase())) ||
    (q.includes('food') || q.includes('dining') || q.includes('eating') ? categoryStats.find((c) => c.category === 'Food & Dining') : undefined) ||
    (q.includes('transport') || q.includes('uber') || q.includes('bolt') || q.includes('ride') || q.includes('fuel') ? categoryStats.find((c) => c.category === 'Transport') : undefined) ||
    (q.includes('shopping') || q.includes('groceries') ? categoryStats.find((c) => c.category === 'Shopping') : undefined)

  if (matchedCat) {
    return `You have spent ${formatKobo(matchedCat.totalKobo)} on ${matchedCat.category} across ${matchedCat.count} transactions (${matchedCat.percent}% of total debits).`
  }

  // 9. Weekend vs weekday
  if (q.includes('weekend') || q.includes('weekday') || q.includes('surge') || q.includes('saturday') || q.includes('sunday')) {
    return `Your weekend spending accounts for ${weekendVsWeekday.weekendPercent}% (${formatKobo(weekendVsWeekday.weekendKobo)}) of total debits, while weekdays account for ${weekendVsWeekday.weekdayPercent}% (${formatKobo(weekendVsWeekday.weekdayKobo)}).`
  }

  // 10. Runway / Burn
  if (q.includes('runway') || q.includes('how long') || q.includes('survive') || q.includes('last me') || q.includes('burn')) {
    return `Your current combined liquid balance is ${formatKobo(totalBalanceKobo)}. At your current daily velocity of ${formatKobo(dailyBurnKobo)} per day, your estimated runway is ${runwayDays} days across all connected banks.`
  }

  // 11. Daily velocity / spend rate
  if (q.includes('velocity') || q.includes('daily spend') || q.includes('per day') || q.includes('rate of spend')) {
    return `Your daily outflow velocity is approximately ${formatKobo(dailyBurnKobo)} per day based on recent transaction activity.`
  }

  // 12. Savings rate / Net cash flow
  if (q.includes('saving') || q.includes('retention') || q.includes('net cash') || q.includes('cash flow')) {
    return `Your net cash flow is ${formatKobo(netCashflowKobo)} with a ${savingsRate}% retention rate (inflow: ${formatKobo(totalInflowKobo)}, outflow: ${formatKobo(totalOutflowKobo)}).`
  }

  // 13. Transaction count / activity volume
  if (q.includes('how many transaction') || q.includes('transaction count') || q.includes('total transaction') || q.includes('activity volume')) {
    return `You have recorded ${stats.incomingCount + stats.outgoingCount} total transactions across all linked accounts (${stats.incomingCount} credits, ${stats.outgoingCount} debits).`
  }

  // 14. Total Outflow / Spend / Nigerian terms ("chop life", "debits")
  if (
    q.includes('total spend') ||
    q.includes('total outflow') ||
    q.includes('total expense') ||
    q.includes('how much did i spend') ||
    q.includes('spent so far') ||
    q.includes('debits') ||
    q.includes('chop life')
  ) {
    return `Your consolidated outflow across all linked accounts is ${formatKobo(totalOutflowKobo)} across ${stats.outgoingCount} debit transactions.`
  }

  // 15. Total Inflow / Deposits / Income / Nigerian terms ("alert", "salary", "credits")
  if (
    q.includes('total inflow') ||
    q.includes('total income') ||
    q.includes('total deposit') ||
    q.includes('how much did i make') ||
    q.includes('how much did i receive') ||
    q.includes('salary') ||
    q.includes('alert') ||
    q.includes('credit')
  ) {
    return `Your total consolidated inflow is ${formatKobo(totalInflowKobo)} across ${stats.incomingCount} credit transactions.`
  }

  // 16. Balance / Nigerian terms ("owo", "funds", "liquid")
  if (q.includes('balance') || q.includes('net worth') || q.includes('how much do i have') || q.includes('owo') || q.includes('funds') || q.includes('pocket') || q.includes('liquid')) {
    return `Your total combined liquid balance is ${formatKobo(totalBalanceKobo)} across ${accountCount || 4} linked bank accounts.`
  }

  // 17. Top Categories Breakdown
  if (q.includes('top') || q.includes('category') || q.includes('breakdown')) {
    const top3 = categoryStats.slice(0, 3)
    if (top3.length === 0) return 'No category records available.'
    const formatted = top3.map((c) => `${c.category} (${formatKobo(c.totalKobo)}, ${c.percent}%)`).join('; ')
    return `Your top spending categories are: ${formatted}. Total consolidated outflow across all rails is ${formatKobo(totalOutflowKobo)}.`
  }

  // Fallback
  return `Across all ${accountCount || 4} linked accounts, total inflow is ${formatKobo(totalInflowKobo)} and total outflow is ${formatKobo(totalOutflowKobo)}, resulting in a net cash flow of ${formatKobo(netCashflowKobo)} (${savingsRate}% retention rate). You can ask me about affordability (e.g. "Can I afford 50k?"), category spending, bank drainage, or recurring bills.`
}
