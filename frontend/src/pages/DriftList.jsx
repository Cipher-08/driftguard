import React from 'react';
import { useQuery } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { ChevronRight } from 'lucide-react';
import axios from '../lib/axios';
import { SeverityBadge, DriftTypeBadge, ProviderBadge } from '../components/Badge';

export default function DriftList() {
  const navigate = useNavigate();
  const { data, isLoading, error } = useQuery({
    queryKey: ['drifts'],
    queryFn: async () => (await axios.get('/api/v1/drifts')).data.drifts || [],
  });

  return (
    <div className="p-8 max-w-6xl mx-auto">
      <h1 className="text-2xl font-bold text-gray-800 mb-1">Detected Drifts</h1>
      <p className="text-gray-500 mb-5">Resources whose live state diverges from their declared (IaC) state.</p>

      <div className="bg-white rounded-xl shadow-sm border border-gray-100 overflow-hidden">
        {isLoading && <div className="p-6 text-gray-500 animate-pulse">Loading drifts…</div>}
        {error && <div className="p-6 text-red-600">Error loading drifts. Please try again.</div>}

        {!isLoading && !error && data && data.length === 0 && (
          <div className="text-gray-500 text-center py-16">
            <div className="text-lg font-medium text-gray-600">All clear ✨</div>
            <div className="text-sm mt-1">No open drift records.</div>
          </div>
        )}

        {!isLoading && !error && data && data.length > 0 && (
          <table className="w-full text-sm">
            <thead>
              <tr className="text-left text-gray-500 bg-gray-50 border-b">
                <th className="py-3 px-4 font-medium">Severity</th>
                <th className="py-3 px-4 font-medium">Type</th>
                <th className="py-3 px-4 font-medium">Resource</th>
                <th className="py-3 px-4 font-medium">Provider</th>
                <th className="py-3 px-4 font-medium">Detected</th>
                <th className="py-3 px-4"></th>
              </tr>
            </thead>
            <tbody>
              {data.map((d) => (
                <tr
                  key={d.id}
                  onClick={() => navigate(`/drifts/${d.id}`)}
                  className="border-b last:border-0 hover:bg-brand-50 cursor-pointer"
                >
                  <td className="py-3 px-4"><SeverityBadge severity={d.severity} /></td>
                  <td className="py-3 px-4"><DriftTypeBadge type={d.drift_type} /></td>
                  <td className="py-3 px-4 font-medium text-gray-700">
                    {d.resource?.resource_name || d.resource?.resource_id || '-'}
                    <span className="text-gray-400 font-normal ml-2">{d.resource?.resource_type}</span>
                  </td>
                  <td className="py-3 px-4"><ProviderBadge provider={d.resource?.provider} /></td>
                  <td className="py-3 px-4 text-gray-500">{new Date(d.created_at).toLocaleString()}</td>
                  <td className="py-3 px-4 text-gray-400"><ChevronRight className="w-4 h-4" /></td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}
