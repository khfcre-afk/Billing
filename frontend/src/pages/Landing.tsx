import { Link } from "react-router-dom";
import { motion } from "framer-motion";
import { useQuery } from "@tanstack/react-query";
import { Rocket, Ticket, ShieldCheck, Cpu, HardDrive, MemoryStick } from "lucide-react";
import { api } from "@/lib/api";
import { formatMoney } from "@/lib/format";

type Tariff = {
  id: string;
  slug: string;
  name: string;
  description: string;
  price_cents: number;
  daily_cost_cents: number;
  cpu_limit: number;
  memory_mb: number;
  disk_mb: number;
};

export default function Landing() {
  const { data: tariffs } = useQuery({
    queryKey: ["tariffs"],
    queryFn: () => api.get<Tariff[]>("/tariffs"),
  });

  return (
    <>
      <section className="py-16 md:py-24">
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.6 }}
          className="max-w-3xl"
        >
          <span className="inline-flex items-center gap-1.5 rounded-full border border-border/70 bg-surface/60 px-3 py-1 text-xs uppercase tracking-wider text-muted">
            <Ticket className="h-3 w-3 text-accent" /> Промо-код биллинг
          </span>
          <h1 className="mt-6 font-display">
            Игровые серверы.{" "}
            <span className="bg-gradient-to-r from-accent to-accent-2 bg-clip-text text-transparent">
              Pterodactyl
            </span>
            . Без платёжек.
          </h1>
          <p className="mt-6 max-w-2xl text-lg text-muted">
            Полнофункциональная панель: активация промокода — мгновенная выдача
            сервера. Автопродление списывает баланс раз в сутки. Платёжные
            шлюзы отключены — весь вход только через промо.
          </p>
          <div className="mt-8 flex flex-wrap gap-3">
            <Link to="/register" className="btn-primary">
              <Rocket className="h-4 w-4" /> Поехали
            </Link>
            <a href="#tariffs" className="btn-ghost">
              Посмотреть тарифы
            </a>
          </div>
          <div className="mt-10 grid gap-4 sm:grid-cols-3">
            <Feature icon={<ShieldCheck className="h-4 w-4" />} title="Безопасность" body="JWT в httpOnly cookie, bcrypt, rate-limit" />
            <Feature icon={<Rocket className="h-4 w-4" />} title="Авто-выдача" body="Pterodactyl User + Server API за секунды" />
            <Feature icon={<Ticket className="h-4 w-4" />} title="Промо-коды" body="Balance / Tariff grant / Percent off" />
          </div>
        </motion.div>
      </section>

      <section id="tariffs" className="py-16">
        <div className="mb-10 flex items-end justify-between">
          <div>
            <h2 className="font-display">Тарифы</h2>
            <p className="mt-2 text-muted">Выбирай, покупай балансом, плати промокодом.</p>
          </div>
        </div>
        <div className="grid gap-5 md:grid-cols-2 lg:grid-cols-3">
          {(tariffs ?? []).map((t) => (
            <motion.div
              key={t.id}
              initial={{ opacity: 0, y: 12 }}
              whileInView={{ opacity: 1, y: 0 }}
              viewport={{ once: true }}
              transition={{ duration: 0.35 }}
              className="card group relative overflow-hidden"
            >
              <div className="pointer-events-none absolute -right-10 -top-10 h-40 w-40 rounded-full bg-accent/20 blur-3xl transition group-hover:bg-accent/30" />
              <h3 className="font-display text-xl font-semibold">{t.name}</h3>
              <p className="mt-1 min-h-10 text-sm text-muted">{t.description}</p>
              <div className="mt-5 flex items-baseline gap-2">
                <span className="font-display text-3xl font-bold">
                  {formatMoney(t.price_cents)}
                </span>
                <span className="text-sm text-muted">
                  / {formatMoney(t.daily_cost_cents)} в день
                </span>
              </div>
              <dl className="mt-5 grid grid-cols-3 gap-2 text-sm">
                <Spec icon={<Cpu className="h-3.5 w-3.5" />} label="CPU" value={`${t.cpu_limit}%`} />
                <Spec icon={<MemoryStick className="h-3.5 w-3.5" />} label="RAM" value={`${t.memory_mb} MB`} />
                <Spec icon={<HardDrive className="h-3.5 w-3.5" />} label="Disk" value={`${t.disk_mb} MB`} />
              </dl>
              <Link to="/register" className="btn-primary mt-6 w-full">
                Получить
              </Link>
            </motion.div>
          ))}
          {(tariffs ?? []).length === 0 && (
            <div className="card col-span-full text-center text-muted">
              Тарифы ещё не настроены. Зайди в админку и создай первый.
            </div>
          )}
        </div>
      </section>

      <section className="py-16">
        <div className="card">
          <h2 className="font-display">FAQ</h2>
          <dl className="mt-6 space-y-4">
            <Faq q="Почему отключены платёжки?" a="Проект использует только промо-коды. Администратор создаёт коды вручную и раздаёт их пользователям." />
            <Faq q="Как работает выдача сервера?" a="После покупки тарифа backend создаёт юзера и сервер в Pterodactyl, выбирает свободную аллокацию и возвращает IP:port." />
            <Faq q="Что если баланс закончится?" a="Сервер уходит в suspended. После пополнения балансом (через промо) вручную снимается с паузы администратором." />
          </dl>
        </div>
      </section>
    </>
  );
}

function Feature({ icon, title, body }: { icon: React.ReactNode; title: string; body: string }) {
  return (
    <div className="card">
      <div className="inline-flex h-8 w-8 items-center justify-center rounded-lg bg-accent/20 text-accent">
        {icon}
      </div>
      <div className="mt-3 font-display font-semibold">{title}</div>
      <div className="mt-1 text-sm text-muted">{body}</div>
    </div>
  );
}

function Spec({ icon, label, value }: { icon: React.ReactNode; label: string; value: string }) {
  return (
    <div className="rounded-xl border border-border/60 bg-surface-2/60 p-2 text-center">
      <div className="flex items-center justify-center gap-1 text-[10px] uppercase tracking-wider text-muted">
        {icon} {label}
      </div>
      <div className="mt-1 text-sm font-semibold">{value}</div>
    </div>
  );
}

function Faq({ q, a }: { q: string; a: string }) {
  return (
    <div>
      <dt className="font-display font-semibold">{q}</dt>
      <dd className="mt-1 text-sm text-muted">{a}</dd>
    </div>
  );
}
