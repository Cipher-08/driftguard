import { useQuery } from '@tanstack/react-query';
import axios from '../lib/axios';

export default function Dashboard() {
  const { data, isLoading, error } = useQuery({
    queryKey: ['summary'],
    queryFn: async () => {
      const res = await axios.get('/api/v1/summary');
      return res.data;
    },
  });

  return (
    <div className="p-8">
      <h1 className="text-3xl font-bold text-brand-700 mb-4">DriftGuard Dashboard</h1>
      {isLoading && <p className="text-gray-600">Loading summary...</p>}
      {error && <p className="text-red-600">Error loading summary.</p>}
      {!isLoading && !error && data && (
        <div className="grid grid-cols-1 md:grid-cols-3 gap-6 mb-8">
          <div className="bg-white rounded shadow p-6 flex flex-col items-center">
            <span className="text-4xl font-bold text-brand-600">{data.total_drifts ?? 0}</span>
            <span className="text-gray-700 mt-2">Total Drifts</span>
          </div>
          <div className="bg-white rounded shadow p-6 flex flex-col items-center">
            <span className="text-4xl font-bold text-green-600">{data.compliant_resources ?? 0}</span>
            <span className="text-gray-700 mt-2">Compliant Resources</span>
          </div>
          <div className="bg-white rounded shadow p-6 flex flex-col items-center">
            <span className="text-4xl font-bold text-red-600">{data.noncompliant_resources ?? 0}</span>
            <span className="text-gray-700 mt-2">Noncompliant Resources</span>
          </div>
        </div>
      )}
      <p className="text-gray-700">Welcome! This is your unified drift and compliance summary.</p>
    </div>
  );
}
