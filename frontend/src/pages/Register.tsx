import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useAuth } from "@/lib/auth";

export default function Register() {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [err, setErr] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const nav = useNavigate();
  const register = useAuth((s) => s.register);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setLoading(true);
    setErr(null);
    try {
      await register(email, password);
      nav("/dashboard");
    } catch (error) {
      setErr((error as Error).message);
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="mx-auto max-w-md">
      <div className="card">
        <h1 className="font-display text-2xl">Регистрация</h1>
        <form onSubmit={onSubmit} className="mt-6 space-y-4">
          <div>
            <label className="label">Email</label>
            <input className="input" type="email" value={email} onChange={(e) => setEmail(e.target.value)} required />
          </div>
          <div>
            <label className="label">Пароль (мин. 8 символов)</label>
            <input className="input" type="password" minLength={8} value={password} onChange={(e) => setPassword(e.target.value)} required />
          </div>
          {err && <div className="text-sm text-danger">{err}</div>}
          <button disabled={loading} className="btn-primary w-full">
            {loading ? "…" : "Создать аккаунт"}
          </button>
        </form>
        <div className="mt-4 text-center text-sm text-muted">
          Уже есть аккаунт? <Link className="text-accent" to="/login">Войти</Link>
        </div>
      </div>
    </div>
  );
}
