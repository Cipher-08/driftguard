import React from 'react';
import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import axios from '../lib/axios';
import { ProviderBadge } from '../components/Badge';

export default function Resources() {
  const { data, isLoading, error } = useQuery({
    queryKey: ['resources'],
    queryFn: async () => (await axios.get('/api/v1/resources')).data.resources || [],
  });

  return (
    <div className="p-8 max-w-6xl mx-auto">
      <h1 className="text-2xl font-bold text-gray-800 mb-1">Resource Inventory</h1>
      <p className="text-gray-500 mb-5">Every cloud resource DriftGuard has discovered across your accounts.</p>

      <div className="bg-white rounded-xl shadow-sm border border-gray-100 overflow-hidden">
        {isLoading && <div className="p-6 text-gray-500 animate-pulse">Loading resources…</div>}
        {error && <div className="p-6 text-red-600">Error loading resources. Please try again.</div>}

        {!isLoading && !error && data && data.length === 0 && (
          <div className="text-gray-500 text-center py-16">
            <div className="text-lg font-medium text-gray-600">No resources yet</div>
            <Link to="/accounts" className="text-brand-600 hover:underline text-sm mt-2 inline-block">
              Connect a cloud account to get started
            </Link>
          </div>
        )}

        {!isLoading && !error && data && data.length > 0 && (
          <table className="w-full text-sm">
            <thead>
              <tr className="text-left text-gray-500 bg-gray-50 border-b">
                <th className="py-3 px-4 font-medium">Type</th>
                <th className="py-3 px-4 font-medium">Name</th>
                <th className="py-3 px-4 font-medium">Region</th>
                <th className="py-3 px-4 font-medium">Provider</th>
                <th className="py-3 px-4 font-medium">Last Scanned</th>
              </tr>
            </thead>
            <tbody>
              {data.map((r) => (
                <tr key={r.id} className="border-b last:border-0 hover:bg-gray-50">
                  <td className="py-3 px-4 font-mono text-gray-600">{r.resource_type}</td>
                  <td className="py-3 px-4 font-medium text-gray-700">{r.resource_name || r.resource_id}</td>
                  <td className="py-3 px-4 text-gray-500">{r.region}</td>
                  <td className="py-3 px-4"><ProviderBadge provider={r.provider} /></td>
                  <td className="py-3 px-4 text-gray-500">
                    {r.last_scanned_at ? new Date(r.last_scanned_at).toLocaleString() : '-'}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}
