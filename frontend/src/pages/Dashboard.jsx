import React from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { RefreshCw, AlertTriangle, ShieldCheck, ShieldAlert, Layers } from 'lucide-react';
import axios from '../lib/axios';

function StatCard({ label, value, accent, icon: Icon, to }) {
  const inner = (
    <div className="bg-white rounded-xl shadow-sm border border-gray-100 p-6 flex items-center gap-4 hover:shadow-md transition h-full">
      {Icon && (
        <div className={`p-3 rounded-lg ${accent?.bg || 'bg-gray-50'}`}>
          <Icon className={`w-6 h-6 ${accent?.text || 'text-gray-500'}`} />
        </div>
      )}
      <div>
        <div className={`text-3xl font-bold ${accent?.text || 'text-gray-800'}`}>{value}</div>
        <div className="text-gray-500 text-sm mt-0.5">{label}</div>
      </div>
    </div>
  );
  return to ? <Link to={to}>{inner}</Link> : inner;
}

export default function Dashboard() {
  const qc = useQueryClient();
  const { data, isLoading, error } = useQuery({
    queryKey: ['summary'],
    queryFn: async () => (await axios.get('/api/v1/summary')).data,
  });

  const scan = useMutation({
    mutationFn: async () => (await axios.post('/api/v1/scan')).data,
    onSuccess: () => setTimeout(() => qc.invalidateQueries({ queryKey: ['summary'] }), 2500),
  });

  return (
    <div className="p-8 max-w-6xl mx-auto">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-3xl font-bold text-gray-800">Dashboard</h1>
          <p className="text-gray-500 mt-1">Unified drift &amp; compliance posture across your clouds.</p>
        </div>
        <button
          onClick={() => scan.mutate()}
          disabled={scan.isPending}
          className="flex items-center gap-2 bg-brand-600 text-white px-4 py-2 rounded-lg hover:bg-brand-700 transition disabled:opacity-50"
        >
          <RefreshCw className={`w-4 h-4 ${scan.isPending ? 'animate-spin' : ''}`} />
          {scan.isPending ? 'Scanning…' : 'Scan now'}
        </button>
      </div>

      {scan.isSuccess && (
        <div className="bg-green-50 border border-green-200 text-green-700 rounded-lg p-3 mb-4 text-sm">
          Scan started — results will refresh shortly.
        </div>
      )}

      {isLoading && <div className="text-gray-500 animate-pulse">Loading summary…</div>}
      {error && (
        <div className="bg-red-50 border border-red-200 text-red-700 rounded-lg p-3 mb-4">
          Error loading summary. Please try again.
        </div>
      )}

      {!isLoading && !error && data && (
        <>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-5 mb-8">
            <StatCard
              label="Open Drifts"
              value={data.total_drifts ?? 0}
              icon={AlertTriangle}
              accent={{ bg: 'bg-purple-50', text: 'text-purple-600' }}
              to="/drifts"
            />
            <StatCard
              label="Affected Resources"
              value={data.affected_resources ?? 0}
              icon={Layers}
              accent={{ bg: 'bg-brand-50', text: 'text-brand-600' }}
            />
            <StatCard
              label="Compliant Resources"
              value={data.compliant_resources ?? 0}
              icon={ShieldCheck}
              accent={{ bg: 'bg-green-50', text: 'text-green-600' }}
            />
            <StatCard
              label="Non-Compliant"
              value={data.noncompliant_resources ?? 0}
              icon={ShieldAlert}
              accent={{ bg: 'bg-red-50', text: 'text-red-600' }}
            />
          </div>

          <h2 className="text-lg font-semibold text-gray-700 mb-3">Drift by severity</h2>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-5">
            <StatCard label="Critical" value={data.critical_drifts ?? 0} accent={{ text: 'text-red-600' }} />
            <StatCard label="High" value={data.high_drifts ?? 0} accent={{ text: 'text-orange-500' }} />
            <StatCard label="Medium" value={data.medium_drifts ?? 0} accent={{ text: 'text-yellow-500' }} />
            <StatCard label="Low" value={data.low_drifts ?? 0} accent={{ text: 'text-blue-500' }} />
          </div>

          {(data.total_resources ?? 0) === 0 && (
            <div className="mt-8 bg-white border border-dashed border-gray-300 rounded-xl p-8 text-center">
              <p className="text-gray-600 mb-3">No resources scanned yet.</p>
              <Link
                to="/accounts"
                className="inline-block bg-brand-600 text-white px-4 py-2 rounded-lg hover:bg-brand-700 transition"
              >
                Connect a cloud account
              </Link>
            </div>
          )}
        </>
      )}
    </div>
  );
}
