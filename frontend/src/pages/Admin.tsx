import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Ticket, Boxes } from "lucide-react";
import { api } from "@/lib/api";
import { formatDate, formatMoney } from "@/lib/format";

type Promo = {
  id: string; code: string; kind: "balance" | "tariff_grant" | "percent_off";
  balance_cents?: number; grant_days?: number; percent_off?: number;
  uses_count: number; max_uses?: number; is_active: boolean; created_at: string;
};

type Tariff = {
  id: string; name: string; slug: string; price_cents: number;
  daily_cost_cents: number; is_active: boolean;
};

export default function Admin() {
  return (
    <div className="space-y-8">
      <h1 className="font-display text-3xl">Админка</h1>
      <AdminPromos />
      <AdminTariffs />
    </div>
  );
}

function AdminPromos() {
  const qc = useQueryClient();
  const { data } = useQuery({ queryKey: ["admin", "promo"], queryFn: () => api.get<Promo[]>("/admin/promo") });
  const [form, setForm] = useState({
    code: "",
    kind: "balance" as Promo["kind"],
    balance_cents: 10000,
    grant_days: 30,
    percent_off: 10,
    max_uses: 100,
  });

  const create = useMutation({
    mutationFn: () => {
      const payload: Record<string, unknown> = {
        code: form.code,
        kind: form.kind,
        max_uses: form.max_uses,
        per_user_limit: 1,
      };
      if (form.kind === "balance") payload.balance_cents = form.balance_cents;
      if (form.kind === "tariff_grant") payload.grant_days = form.grant_days;
      if (form.kind === "percent_off") payload.percent_off = form.percent_off;
      return api.post("/admin/promo", payload);
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["admin", "promo"] }),
  });

  return (
    <div className="card">
      <div className="flex items-center gap-2">
        <Ticket className="h-4 w-4 text-accent" />
        <h2 className="font-display text-xl">Промокоды</h2>
      </div>
      <div className="mt-4 grid gap-3 md:grid-cols-5">
        <input className="input md:col-span-2" placeholder="CODE" value={form.code} onChange={(e) => setForm({ ...form, code: e.target.value.toUpperCase() })} />
        <select className="input" value={form.kind} onChange={(e) => setForm({ ...form, kind: e.target.value as Promo["kind"] })}>
          <option value="balance">Balance</option>
          <option value="tariff_grant">Tariff grant</option>
          <option value="percent_off">Percent off</option>
        </select>
        {form.kind === "balance" && (
          <input className="input" type="number" placeholder="cents" value={form.balance_cents} onChange={(e) => setForm({ ...form, balance_cents: +e.target.value })} />
        )}
        {form.kind === "tariff_grant" && (
          <input className="input" type="number" placeholder="days" value={form.grant_days} onChange={(e) => setForm({ ...form, grant_days: +e.target.value })} />
        )}
        {form.kind === "percent_off" && (
          <input className="input" type="number" placeholder="%" value={form.percent_off} onChange={(e) => setForm({ ...form, percent_off: +e.target.value })} />
        )}
        <button className="btn-primary" disabled={!form.code || create.isPending} onClick={() => create.mutate()}>
          Создать
        </button>
      </div>
      <div className="mt-6 overflow-x-auto">
        <table className="w-full text-sm">
          <thead className="text-left text-muted">
            <tr>
              <th className="py-2">Код</th>
              <th>Тип</th>
              <th>Условия</th>
              <th>Использовано</th>
              <th>Создан</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-border/60">
            {(data ?? []).map((p) => (
              <tr key={p.id}>
                <td className="py-2 font-mono">{p.code}</td>
                <td>{p.kind}</td>
                <td>
                  {p.kind === "balance" && formatMoney(p.balance_cents ?? 0)}
                  {p.kind === "tariff_grant" && `${p.grant_days} дней`}
                  {p.kind === "percent_off" && `${p.percent_off}%`}
                </td>
                <td>{p.uses_count}{p.max_uses ? ` / ${p.max_uses}` : ""}</td>
                <td className="text-muted">{formatDate(p.created_at)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function AdminTariffs() {
  const qc = useQueryClient();
  const { data } = useQuery({ queryKey: ["tariffs", "admin"], queryFn: () => api.get<Tariff[]>("/tariffs") });
  const [form, setForm] = useState({
    slug: "", name: "", description: "",
    price_cents: 50000, daily_cost_cents: 1000,
    cpu_limit: 100, memory_mb: 1024, disk_mb: 5120, swap_mb: 0, io_weight: 500,
    egg_id: 1, nest_id: 1,
    docker_image: "ghcr.io/pterodactyl/yolks:java_17",
    startup_command: "java -Xms128M -Xmx{{SERVER_MEMORY}}M -jar server.jar",
  });
  const create = useMutation({
    mutationFn: () => api.post("/admin/tariffs", { ...form, environment: {} }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["tariffs"] }),
  });

  return (
    <div className="card">
      <div className="flex items-center gap-2">
        <Boxes className="h-4 w-4 text-accent" />
        <h2 className="font-display text-xl">Тарифы</h2>
      </div>
      <div className="mt-4 grid gap-3 md:grid-cols-4">
        <input className="input" placeholder="slug" value={form.slug} onChange={(e) => setForm({ ...form, slug: e.target.value })} />
        <input className="input md:col-span-2" placeholder="Название" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} />
        <input className="input" type="number" placeholder="price cents" value={form.price_cents} onChange={(e) => setForm({ ...form, price_cents: +e.target.value })} />
        <input className="input" type="number" placeholder="daily cents" value={form.daily_cost_cents} onChange={(e) => setForm({ ...form, daily_cost_cents: +e.target.value })} />
        <input className="input" type="number" placeholder="RAM MB" value={form.memory_mb} onChange={(e) => setForm({ ...form, memory_mb: +e.target.value })} />
        <input className="input" type="number" placeholder="Disk MB" value={form.disk_mb} onChange={(e) => setForm({ ...form, disk_mb: +e.target.value })} />
        <input className="input" type="number" placeholder="CPU %" value={form.cpu_limit} onChange={(e) => setForm({ ...form, cpu_limit: +e.target.value })} />
        <input className="input" type="number" placeholder="egg_id" value={form.egg_id} onChange={(e) => setForm({ ...form, egg_id: +e.target.value })} />
        <input className="input" type="number" placeholder="nest_id" value={form.nest_id} onChange={(e) => setForm({ ...form, nest_id: +e.target.value })} />
        <input className="input md:col-span-2" placeholder="docker image" value={form.docker_image} onChange={(e) => setForm({ ...form, docker_image: e.target.value })} />
        <input className="input md:col-span-4" placeholder="startup" value={form.startup_command} onChange={(e) => setForm({ ...form, startup_command: e.target.value })} />
        <button className="btn-primary md:col-span-4" disabled={!form.slug || !form.name || create.isPending} onClick={() => create.mutate()}>
          Создать тариф
        </button>
      </div>
      <div className="mt-6 overflow-x-auto">
        <table className="w-full text-sm">
          <thead className="text-left text-muted">
            <tr>
              <th className="py-2">Slug</th>
              <th>Name</th>
              <th>Price</th>
              <th>Daily</th>
              <th>Active</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-border/60">
            {(data ?? []).map((t) => (
              <tr key={t.id}>
                <td className="py-2 font-mono">{t.slug}</td>
                <td>{t.name}</td>
                <td>{formatMoney(t.price_cents)}</td>
                <td>{formatMoney(t.daily_cost_cents)}</td>
                <td>{t.is_active ? "yes" : "no"}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
