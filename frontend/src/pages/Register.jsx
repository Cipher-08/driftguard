import React, { useState } from 'react';
import axios from '../lib/axios';
import { Link, useNavigate, Navigate } from 'react-router-dom';
import { ShieldCheck } from 'lucide-react';
import { setSession, isAuthenticated } from '../lib/auth';

export default function Register() {
  const [form, setForm] = useState({ org_name: '', name: '', email: '', password: '' });
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const navigate = useNavigate();

  if (isAuthenticated()) return <Navigate to="/dashboard" replace />;

  const update = (k) => (e) => setForm({ ...form, [k]: e.target.value });

  const slugify = (s) =>
    s.toLowerCase().trim().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '') || 'org';

  const handleSubmit = async (e) => {
    e.preventDefault();
    setLoading(true);
    setError('');
    try {
      const res = await axios.post('/api/v1/auth/register', {
        ...form,
        org_slug: slugify(form.org_name),
      });
      setSession(res.data.token, res.data.user);
      navigate('/accounts');
    } catch (err) {
      setError(err.response?.data?.error || 'Registration failed');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-gradient-to-br from-brand-50 to-brand-100">
      <div className="bg-white p-8 rounded-xl shadow-lg w-full max-w-sm">
        <div className="flex items-center gap-2 mb-6">
          <ShieldCheck className="w-7 h-7 text-brand-600" />
          <h2 className="text-2xl font-bold text-brand-700">Create your account</h2>
        </div>
        <form onSubmit={handleSubmit} className="space-y-4">
          <input
            className="w-full p-2.5 border rounded-lg focus:outline-none focus:ring-2 focus:ring-brand-300"
            placeholder="Organization name"
            value={form.org_name}
            onChange={update('org_name')}
            required
          />
          <input
            className="w-full p-2.5 border rounded-lg focus:outline-none focus:ring-2 focus:ring-brand-300"
            placeholder="Your name"
            value={form.name}
            onChange={update('name')}
            required
          />
          <input
            className="w-full p-2.5 border rounded-lg focus:outline-none focus:ring-2 focus:ring-brand-300"
            type="email"
            placeholder="Email"
            value={form.email}
            onChange={update('email')}
            required
          />
          <input
            className="w-full p-2.5 border rounded-lg focus:outline-none focus:ring-2 focus:ring-brand-300"
            type="password"
            placeholder="Password (min 8 chars)"
            value={form.password}
            onChange={update('password')}
            minLength={8}
            required
          />
          {error && <div className="text-red-600 text-sm">{error}</div>}
          <button
            className="w-full bg-brand-600 text-white py-2.5 rounded-lg hover:bg-brand-700 transition disabled:opacity-50 font-semibold"
            disabled={loading}
            type="submit"
          >
            {loading ? 'Creating…' : 'Create account'}
          </button>
        </form>
        <p className="text-sm text-gray-500 mt-5 text-center">
          Already have an account?{' '}
          <Link to="/login" className="text-brand-600 font-medium hover:underline">
            Sign in
          </Link>
        </p>
      </div>
    </div>
  );
}
