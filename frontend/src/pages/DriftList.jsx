import { useQuery } from '@tanstack/react-query';
import axios from '../lib/axios';

export default function DriftList() {
  const { data, isLoading, error } = useQuery({
    queryKey: ['drifts'],
    queryFn: async () => {
      const res = await axios.get('/api/v1/drifts');
      return res.data.drifts || [];
    },
  });

  return (
    <div className="p-8 max-w-5xl mx-auto">
      <h1 className="text-2xl font-bold text-brand-700 mb-4">Detected Drifts</h1>
      <div className="bg-white p-4 rounded shadow overflow-x-auto">
        {isLoading && (
          <div className="flex items-center gap-2 text-gray-600 animate-pulse">
            <svg className="w-5 h-5 animate-spin" fill="none" viewBox="0 0 24 24"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8z"></path></svg>
            Loading drifts...
          </div>
        )}
        {error && (
          <div className="bg-red-50 border border-red-200 text-red-700 rounded p-3 mb-4">Error loading drifts. Please try again.</div>
        )}
        {!isLoading && !error && data && data.length === 0 && (
          <div className="text-gray-500 text-center py-8">
            <svg className="w-12 h-12 mx-auto mb-2 text-gray-300" fill="none" viewBox="0 0 24 24"><path stroke="currentColor" strokeWidth="2" d="M12 4v16m8-8H4"/></svg>
            <div>No drift records found. All clear!</div>
          </div>
        )}
        {!isLoading && !error && data && data.length > 0 && (
          <table className="w-full text-sm">
            <thead>
              <tr className="text-left border-b">
                <th className="py-2">Type</th>
                <th className="py-2">Severity</th>
                <th className="py-2">Resource</th>
                <th className="py-2">Changed At</th>
                <th className="py-2">Resolved</th>
              </tr>
            </thead>
            <tbody>
              {data.map((drift) => (
                <tr key={drift.id} className="border-b hover:bg-brand-50">
                  <td className="py-2">{drift.drift_type}</td>
                  <td className="py-2">{drift.severity}</td>
                  <td className="py-2">{drift.resource?.resource_name || '-'}</td>
                  <td className="py-2">{drift.changed_at ? new Date(drift.changed_at).toLocaleString() : '-'}</td>
                  <td className="py-2">{drift.is_resolved ? 'Yes' : 'No'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
      {/* Toast notifications placeholder */}
    </div>
  );
}
