export type CompanionMetrics = {
  duration_ms: number;
  model_calls: number;
  reported_calls: number;
  usage?: {prompt_tokens: number; completion_tokens: number; total_tokens: number};
};

export function CompanionCost({metrics, label = "此条生成记录"}: {metrics?: CompanionMetrics; label?: string}) {
  if (!metrics) return null;
  const complete = metrics.model_calls > 0 && metrics.reported_calls === metrics.model_calls && !!metrics.usage;
  return <details className="cr-cost">
    <summary>{label} · {(metrics.duration_ms / 1000).toFixed(1)} 秒 · {metrics.model_calls} 次模型调用</summary>
    {complete ? <p>输入 {metrics.usage!.prompt_tokens} · 输出 {metrics.usage!.completion_tokens} · 合计 {metrics.usage!.total_tokens} Token</p>
      : <p>{metrics.model_calls === 0 ? "本次未调用模型。" : `Token 未完整返回（${metrics.reported_calls}/${metrics.model_calls} 次），不估算总用量。`}</p>}
    <p>耗时包含排队与核对；Token 来自网关回执，不代表账单金额。历史记录显示生成当时的用量，重读不重复计费。</p>
  </details>;
}
