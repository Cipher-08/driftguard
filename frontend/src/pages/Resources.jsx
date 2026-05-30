import { useQuery } from '@tanstack/react-query';
import axios from '../lib/axios';

export default function Resources() {
  const { data, isLoading, error } = useQuery({
    queryKey: ['resources'],
    queryFn: async () => {
      const res = await axios.get('/api/v1/resources');
      return res.data.resources || [];
    },
  });

  return (
    <div className="p-8">
      <h1 className="text-2xl font-bold text-brand-700 mb-4">Resource Inventory</h1>
      <div className="bg-white p-4 rounded shadow">
        {isLoading && <p className="text-gray-600">Loading...</p>}
        {error && <p className="text-red-600">Error loading resources.</p>}
        {!isLoading && !error && data && data.length === 0 && (
          <p className="text-gray-600">No resources found.</p>
        )}
        {!isLoading && !error && data && data.length > 0 && (
          <table className="w-full text-sm">
            <thead>
              <tr className="text-left border-b">
                <th className="py-2">Type</th>
                <th className="py-2">Name</th>
                <th className="py-2">Region</th>
                <th className="py-2">Provider</th>
                <th className="py-2">Last Scanned</th>
              </tr>
            </thead>
            <tbody>
              {data.map((r) => (
                <tr key={r.id} className="border-b hover:bg-brand-50">
                  <td className="py-2">{r.resource_type}</td>
                  <td className="py-2">{r.resource_name}</td>
                  <td className="py-2">{r.region}</td>
                  <td className="py-2">{r.provider}</td>
                  <td className="py-2">{r.last_scanned_at ? new Date(r.last_scanned_at).toLocaleString() : '-'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}
