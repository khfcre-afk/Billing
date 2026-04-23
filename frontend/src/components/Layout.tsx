import { useEffect, type ReactNode } from "react";
import { Link, useLocation } from "react-router-dom";
import { motion } from "framer-motion";
import { LogOut, Shield, Zap } from "lucide-react";
import { useAuth } from "@/lib/auth";
import { formatMoney } from "@/lib/format";
import { cn } from "@/lib/cn";

export default function Layout({ children }: { children: ReactNode }) {
  const { me, bootstrap, logout } = useAuth();
  const { pathname } = useLocation();

  useEffect(() => {
    void bootstrap();
  }, [bootstrap]);

  return (
    <div className="min-h-screen">
      <header className="sticky top-0 z-30 backdrop-blur-xl bg-bg/60 border-b border-border/60">
        <div className="container flex h-16 items-center justify-between">
          <Link to="/" className="flex items-center gap-2 font-display text-lg font-semibold">
            <span className="inline-flex h-8 w-8 items-center justify-center rounded-lg bg-accent/20 text-accent">
              <Zap className="h-4 w-4" />
            </span>
            PteroBilling
          </Link>
          <nav className="flex items-center gap-2">
            {me ? (
              <>
                <div className="hidden md:flex items-center gap-2 rounded-xl bg-surface-2/70 border border-border/60 px-3 py-1.5 text-sm">
                  <span className="text-muted">Баланс:</span>
                  <span className="font-semibold">{formatMoney(me.balance_cents)}</span>
                </div>
                <NavLink to="/dashboard" active={pathname.startsWith("/dashboard")}>
                  Кабинет
                </NavLink>
                {me.role === "admin" && (
                  <NavLink to="/admin" active={pathname.startsWith("/admin")}>
                    <Shield className="h-3.5 w-3.5" /> Админка
                  </NavLink>
                )}
                <button className="btn-ghost" onClick={() => logout()}>
                  <LogOut className="h-4 w-4" /> Выйти
                </button>
              </>
            ) : (
              <>
                <NavLink to="/login" active={pathname === "/login"}>Вход</NavLink>
                <Link to="/register" className="btn-primary">Регистрация</Link>
              </>
            )}
          </nav>
        </div>
      </header>
      <motion.main
        key={pathname}
        initial={{ opacity: 0, y: 8 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.25 }}
        className="container py-10"
      >
        {children}
      </motion.main>
      <footer className="border-t border-border/60 py-10 text-center text-sm text-muted">
        © PteroBilling · Space Grotesk × Inter · Промо-код биллинг
      </footer>
    </div>
  );
}

function NavLink({
  to,
  active,
  children,
}: {
  to: string;
  active: boolean;
  children: ReactNode;
}) {
  return (
    <Link
      to={to}
      className={cn(
        "btn-ghost",
        active && "border-accent/60 text-white bg-surface shadow-glow",
      )}
    >
      {children}
    </Link>
  );
}
