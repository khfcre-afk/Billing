import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Ticket, Server as ServerIcon, Wallet, Play, Square, RotateCcw, Plus } from "lucide-react";
import { api } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { formatDate, formatMoney } from "@/lib/format";

type Tariff = {
  id: string; name: string; description: string;
  price_cents: number; daily_cost_cents: number;
  cpu_limit: number; memory_mb: number; disk_mb: number;
};

type Server = {
  id: string; name: string; status: string;
  paid_until: string; allocation_ip?: string; allocation_port?: number;
};

type Tx = {
  id: string; kind: string; amount_cents: number; balance_after: number;
  description: string; created_at: string;
};

export default function Dashboard() {
  const { me, refresh } = useAuth();
  const qc = useQueryClient();
  const [code, setCode] = useState("");
  const [notice, setNotice] = useState<string | null>(null);

  const servers = useQuery({ queryKey: ["servers"], queryFn: () => api.get<Server[]>("/servers") });
  const txs = useQuery({ queryKey: ["transactions"], queryFn: () => api.get<Tx[]>("/transactions?limit=30") });
  const tariffs = useQuery({ queryKey: ["tariffs"], queryFn: () => api.get<Tariff[]>("/tariffs") });

  const redeem = useMutation({
    mutationFn: (c: string) => api.post<{ message: string }>("/promo/redeem", { code: c }),
    onSuccess: async (res) => {
      setNotice(res.message);
      setCode("");
      await refresh();
      void qc.invalidateQueries({ queryKey: ["servers"] });
      void qc.invalidateQueries({ queryKey: ["transactions"] });
    },
    onError: (e: Error) => setNotice(e.message),
  });

  const purchase = useMutation({
    mutationFn: (id: string) => api.post(`/tariffs/${id}/purchase`),
    onSuccess: async () => {
      setNotice("Сервер создан");
      await refresh();
      void qc.invalidateQueries({ queryKey: ["servers"] });
      void qc.invalidateQueries({ queryKey: ["transactions"] });
    },
    onError: (e: Error) => setNotice(e.message),
  });

  const power = useMutation({
    mutationFn: (v: { id: string; signal: "start" | "stop" | "restart" }) =>
      api.post(`/servers/${v.id}/power`, { signal: v.signal }),
    onSuccess: () => setNotice("Команда отправлена"),
    onError: (e: Error) => setNotice(e.message),
  });

  return (
    <div className="grid gap-6 lg:grid-cols-[1fr_360px]">
      <div className="space-y-6">
        <div className="card">
          <div className="flex items-center gap-3">
            <div className="inline-flex h-10 w-10 items-center justify-center rounded-xl bg-accent/20 text-accent">
              <Wallet className="h-5 w-5" />
            </div>
            <div>
              <div className="text-xs uppercase tracking-wider text-muted">Баланс</div>
              <div className="font-display text-3xl font-bold">
                {me ? formatMoney(me.balance_cents) : "…"}
              </div>
            </div>
          </div>
        </div>

        <div className="card">
          <div className="flex items-center gap-2">
            <ServerIcon className="h-4 w-4 text-accent" />
            <h2 className="font-display text-xl">Мои серверы</h2>
          </div>
          <div className="mt-4 space-y-3">
            {(servers.data ?? []).map((s) => (
              <div key={s.id} className="flex flex-wrap items-center gap-3 rounded-xl border border-border/60 bg-surface-2/60 p-4">
                <div className="flex-1">
                  <div className="font-semibold">{s.name}</div>
                  <div className="text-xs text-muted">
                    {s.allocation_ip ? `${s.allocation_ip}:${s.allocation_port}` : "—"} · оплачено до {formatDate(s.paid_until)}
                  </div>
                </div>
                <span className={`rounded-full px-2 py-0.5 text-xs ${s.status === "active" ? "bg-success/20 text-success" : "bg-danger/20 text-danger"}`}>
                  {s.status}
                </span>
                <div className="flex items-center gap-1">
                  <button className="btn-ghost" title="Start" onClick={() => power.mutate({ id: s.id, signal: "start" })}>
                    <Play className="h-3.5 w-3.5" />
                  </button>
                  <button className="btn-ghost" title="Stop" onClick={() => power.mutate({ id: s.id, signal: "stop" })}>
                    <Square className="h-3.5 w-3.5" />
                  </button>
                  <button className="btn-ghost" title="Restart" onClick={() => power.mutate({ id: s.id, signal: "restart" })}>
                    <RotateCcw className="h-3.5 w-3.5" />
                  </button>
                </div>
              </div>
            ))}
            {(servers.data ?? []).length === 0 && (
              <div className="text-sm text-muted">У тебя пока нет серверов.</div>
            )}
          </div>
        </div>

        <div className="card">
          <h2 className="font-display text-xl">История транзакций</h2>
          <div className="mt-4 divide-y divide-border/60">
            {(txs.data ?? []).map((t) => (
              <div key={t.id} className="flex items-center justify-between py-2.5 text-sm">
                <div>
                  <div className="font-medium">{t.description}</div>
                  <div className="text-xs text-muted">{formatDate(t.created_at)} · {t.kind}</div>
                </div>
                <div className={`font-semibold ${t.amount_cents > 0 ? "text-success" : t.amount_cents < 0 ? "text-danger" : "text-muted"}`}>
                  {t.amount_cents >= 0 ? "+" : ""}
                  {formatMoney(t.amount_cents)}
                </div>
              </div>
            ))}
            {(txs.data ?? []).length === 0 && <div className="text-sm text-muted">Пока пусто.</div>}
          </div>
        </div>
      </div>

      <aside className="space-y-6">
        <div className="card">
          <div className="flex items-center gap-2">
            <Ticket className="h-4 w-4 text-accent" />
            <h2 className="font-display text-xl">Промо-код</h2>
          </div>
          <form
            className="mt-4 space-y-3"
            onSubmit={(e) => {
              e.preventDefault();
              if (code.trim()) redeem.mutate(code.trim());
            }}
          >
            <input className="input" placeholder="PROMO-XXXX" value={code} onChange={(e) => setCode(e.target.value.toUpperCase())} />
            <button disabled={redeem.isPending || code.trim().length < 3} className="btn-primary w-full">
              Активировать
            </button>
          </form>
          {notice && <div className="mt-3 text-xs text-muted">{notice}</div>}
        </div>

        <div className="card">
          <h2 className="font-display text-xl">Тарифы</h2>
          <div className="mt-3 space-y-3">
            {(tariffs.data ?? []).map((t) => (
              <div key={t.id} className="rounded-xl border border-border/60 bg-surface-2/60 p-3">
                <div className="flex items-center justify-between">
                  <div className="font-semibold">{t.name}</div>
                  <div className="text-sm">{formatMoney(t.price_cents)}</div>
                </div>
                <div className="mt-1 text-xs text-muted">
                  {t.cpu_limit}% CPU · {t.memory_mb} MB · {t.disk_mb} MB
                </div>
                <button
                  disabled={!me || me.balance_cents < t.price_cents || purchase.isPending}
                  onClick={() => purchase.mutate(t.id)}
                  className="btn-primary mt-3 w-full disabled:opacity-50"
                >
                  <Plus className="h-4 w-4" /> Купить
                </button>
              </div>
            ))}
          </div>
        </div>
      </aside>
    </div>
  );
}
