import React, { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Trash2, Cloud, Plus } from 'lucide-react';
import axios from '../lib/axios';
import { ProviderBadge } from '../components/Badge';

function AwsForm({ onSubmit, pending }) {
  const [f, setF] = useState({ name: '', access_key_id: '', secret_access_key: '', region: 'us-east-1' });
  const submit = (e) => {
    e.preventDefault();
    onSubmit({
      provider: 'aws',
      name: f.name,
      credentials: { access_key_id: f.access_key_id, secret_access_key: f.secret_access_key, region: f.region },
    });
  };
  return (
    <form onSubmit={submit} className="space-y-3">
      <input className="w-full p-2 border rounded-lg text-sm" placeholder="Account name (e.g. Prod)"
        value={f.name} onChange={(e) => setF({ ...f, name: e.target.value })} required />
      <input className="w-full p-2 border rounded-lg text-sm" placeholder="Access Key ID"
        value={f.access_key_id} onChange={(e) => setF({ ...f, access_key_id: e.target.value })} required />
      <input className="w-full p-2 border rounded-lg text-sm font-mono" type="password" placeholder="Secret Access Key"
        value={f.secret_access_key} onChange={(e) => setF({ ...f, secret_access_key: e.target.value })} required />
      <input className="w-full p-2 border rounded-lg text-sm" placeholder="Region"
        value={f.region} onChange={(e) => setF({ ...f, region: e.target.value })} required />
      <button disabled={pending} className="w-full bg-brand-600 text-white py-2 rounded-lg hover:bg-brand-700 disabled:opacity-50">
        {pending ? 'Connecting…' : 'Connect AWS'}
      </button>
    </form>
  );
}

function GcpForm({ onSubmit, pending }) {
  const [name, setName] = useState('');
  const [jsonKey, setJsonKey] = useState('');
  const [err, setErr] = useState('');
  const submit = (e) => {
    e.preventDefault();
    let credentials;
    try {
      credentials = JSON.parse(jsonKey);
    } catch {
      setErr('Invalid JSON key.');
      return;
    }
    setErr('');
    onSubmit({ provider: 'gcp', name, credentials });
  };
  return (
    <form onSubmit={submit} className="space-y-3">
      <input className="w-full p-2 border rounded-lg text-sm" placeholder="Account name (e.g. GCP Prod)"
        value={name} onChange={(e) => setName(e.target.value)} required />
      <textarea className="w-full p-2 border rounded-lg font-mono text-xs h-32" placeholder="Paste service-account JSON key"
        value={jsonKey} onChange={(e) => setJsonKey(e.target.value)} required />
      {err && <div className="text-red-600 text-sm">{err}</div>}
      <button disabled={pending} className="w-full bg-brand-600 text-white py-2 rounded-lg hover:bg-brand-700 disabled:opacity-50">
        {pending ? 'Connecting…' : 'Connect GCP'}
      </button>
    </form>
  );
}

export default function Accounts() {
  const qc = useQueryClient();
  const [tab, setTab] = useState('aws');

  const { data, isLoading } = useQuery({
    queryKey: ['credentials'],
    queryFn: async () => (await axios.get('/api/v1/credentials')).data.accounts || [],
  });

  const add = useMutation({
    mutationFn: async (payload) => (await axios.post('/api/v1/credentials', payload)).data,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['credentials'] }),
  });

  const remove = useMutation({
    mutationFn: async (cid) => (await axios.delete(`/api/v1/credentials/${cid}`)).data,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['credentials'] }),
  });

  return (
    <div className="p-8 max-w-5xl mx-auto">
      <h1 className="text-2xl font-bold text-gray-800 mb-1">Cloud Accounts</h1>
      <p className="text-gray-500 mb-6">Connect read-only credentials. DriftGuard scans them on a schedule.</p>

      <div className="grid md:grid-cols-2 gap-6">
        {/* Connected accounts */}
        <div className="bg-white border border-gray-100 rounded-xl p-6 shadow-sm">
          <h2 className="font-semibold text-gray-700 mb-4 flex items-center gap-2">
            <Cloud className="w-5 h-5 text-brand-600" /> Connected
          </h2>
          {isLoading && <div className="text-gray-500 animate-pulse">Loading…</div>}
          {!isLoading && (!data || data.length === 0) && (
            <div className="text-gray-400 text-sm py-6 text-center">No accounts connected yet.</div>
          )}
          <ul className="space-y-2">
            {data?.map((a) => (
              <li key={a.id} className="flex items-center justify-between border rounded-lg p-3">
                <div className="flex items-center gap-3">
                  <ProviderBadge provider={a.provider} />
                  <span className="font-medium text-gray-700">{a.name}</span>
                </div>
                <button onClick={() => remove.mutate(a.id)} className="text-gray-400 hover:text-red-600">
                  <Trash2 className="w-4 h-4" />
                </button>
              </li>
            ))}
          </ul>
        </div>

        {/* Add new */}
        <div className="bg-white border border-gray-100 rounded-xl p-6 shadow-sm">
          <h2 className="font-semibold text-gray-700 mb-4 flex items-center gap-2">
            <Plus className="w-5 h-5 text-brand-600" /> Connect a new account
          </h2>
          <div className="flex gap-2 mb-4">
            {['aws', 'gcp'].map((p) => (
              <button
                key={p}
                onClick={() => setTab(p)}
                className={`px-3 py-1.5 rounded-lg text-sm font-medium uppercase ${
                  tab === p ? 'bg-brand-600 text-white' : 'bg-gray-100 text-gray-600'
                }`}
              >
                {p}
              </button>
            ))}
          </div>
          {tab === 'aws' ? (
            <AwsForm onSubmit={add.mutate} pending={add.isPending} />
          ) : (
            <GcpForm onSubmit={add.mutate} pending={add.isPending} />
          )}
          {add.isSuccess && <div className="text-green-600 text-sm mt-3">Connected! Scanning in the background…</div>}
          {add.isError && (
            <div className="text-red-600 text-sm mt-3">{add.error?.response?.data?.error || 'Failed to connect.'}</div>
          )}
        </div>
      </div>
    </div>
  );
}
