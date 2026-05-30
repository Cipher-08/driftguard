import React, { useState } from 'react';
import axios from '../lib/axios';
import { useNavigate } from 'react-router-dom';

export default function AddGcpCredential() {
  const [name, setName] = useState('');
  const [jsonKey, setJsonKey] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState(false);
  const navigate = useNavigate();

  const handleSubmit = async (e) => {
    e.preventDefault();
    setLoading(true);
    setError('');
    setSuccess(false);
    let credentials;
    try {
      credentials = JSON.parse(jsonKey);
    } catch (err) {
      setError('Invalid JSON key format.');
      setLoading(false);
      return;
    }
    try {
      await axios.post('/api/v1/credentials', {
        provider: 'gcp',
        name,
        credentials,
      });
      setSuccess(true);
      setTimeout(() => navigate('/resources'), 1200);
    } catch (err) {
      setError(err.response?.data?.error || 'Failed to add credentials.');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-brand-50">
      <div className="bg-white p-8 rounded shadow w-full max-w-md">
        <h2 className="text-2xl font-bold mb-4 text-brand-700 flex items-center gap-2">
          <span className="inline-block bg-yellow-100 text-yellow-700 rounded px-2 py-1 text-sm">GCP</span>
          Connect Google Cloud Account
        </h2>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block mb-1 font-medium text-brand-700">Account Name</label>
            <input
              className="w-full p-2 border rounded focus:outline-none focus:ring-2 focus:ring-brand-300"
              type="text"
              placeholder="e.g. My GCP Prod"
              value={name}
              onChange={e => setName(e.target.value)}
              required
            />
          </div>
          <div>
            <label className="block mb-1 font-medium text-brand-700">Service Account JSON Key</label>
            <textarea
              className="w-full p-2 border rounded font-mono text-xs h-32 focus:outline-none focus:ring-2 focus:ring-brand-300"
              placeholder="Paste your GCP service account JSON key here"
              value={jsonKey}
              onChange={e => setJsonKey(e.target.value)}
              required
            />
            <p className="text-xs text-gray-500 mt-1">Get this from Google Cloud Console &rarr; IAM &amp; Admin &rarr; Service Accounts &rarr; Create Key (JSON)</p>
          </div>
          {error && <div className="text-red-600 text-sm">{error}</div>}
          {success && <div className="text-green-600 text-sm">GCP credentials added! Redirecting...</div>}
          <button
            className="w-full bg-brand-600 text-white py-2 rounded hover:bg-brand-700 transition disabled:opacity-50 font-semibold text-lg"
            disabled={loading}
            type="submit"
          >
            {loading ? 'Connecting...' : 'Connect GCP Account'}
          </button>
        </form>
      </div>
    </div>
  );
}
