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
    <div className="p-8">
      <h1 className="text-2xl font-bold text-brand-700 mb-4">Detected Drifts</h1>
      <div className="bg-white p-4 rounded shadow">
        {isLoading && <p className="text-gray-600">Loading...</p>}
        {error && <p className="text-red-600">Error loading drifts.</p>}
        {!isLoading && !error && data && data.length === 0 && (
          <p className="text-gray-600">No drift records found.</p>
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
    </div>
  );
}
