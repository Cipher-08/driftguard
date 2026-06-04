import React, { useState } from 'react';
import axios from '../lib/axios';
import { Link, useNavigate, Navigate } from 'react-router-dom';
import { ShieldCheck } from 'lucide-react';
import { setSession, isAuthenticated } from '../lib/auth';

export default function Login() {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const navigate = useNavigate();

  if (isAuthenticated()) return <Navigate to="/dashboard" replace />;

  const handleSubmit = async (e) => {
    e.preventDefault();
    setLoading(true);
    setError('');
    try {
      const res = await axios.post('/api/v1/auth/login', { email, password });
      setSession(res.data.token, res.data.user);
      navigate('/dashboard');
    } catch (err) {
      setError(err.response?.data?.error || 'Login failed');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-gradient-to-br from-brand-50 to-brand-100">
      <div className="bg-white p-8 rounded-xl shadow-lg w-full max-w-sm">
        <div className="flex items-center gap-2 mb-6">
          <ShieldCheck className="w-7 h-7 text-brand-600" />
          <h2 className="text-2xl font-bold text-brand-700">DriftGuard</h2>
        </div>
        <p className="text-gray-500 mb-6 text-sm">Sign in to your account</p>
        <form onSubmit={handleSubmit}>
          <input
            className="w-full mb-4 p-2.5 border rounded-lg focus:outline-none focus:ring-2 focus:ring-brand-300"
            type="email"
            placeholder="Email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
          />
          <input
            className="w-full mb-4 p-2.5 border rounded-lg focus:outline-none focus:ring-2 focus:ring-brand-300"
            type="password"
            placeholder="Password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
          />
          {error && <div className="text-red-600 mb-3 text-sm">{error}</div>}
          <button
            className="w-full bg-brand-600 text-white py-2.5 rounded-lg hover:bg-brand-700 transition disabled:opacity-50 font-semibold"
            disabled={loading}
            type="submit"
          >
            {loading ? 'Signing in…' : 'Sign in'}
          </button>
        </form>
        <p className="text-sm text-gray-500 mt-5 text-center">
          No account?{' '}
          <Link to="/register" className="text-brand-600 font-medium hover:underline">
            Create one
          </Link>
        </p>
      </div>
    </div>
  );
}
