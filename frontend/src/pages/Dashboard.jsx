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
    <div className="p-8 max-w-4xl mx-auto">
      <h1 className="text-3xl font-bold text-brand-700 mb-4">DriftGuard Dashboard</h1>
      {isLoading && (
        <div className="flex items-center gap-2 text-gray-600 animate-pulse">
          <svg className="w-5 h-5 animate-spin" fill="none" viewBox="0 0 24 24"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8z"></path></svg>
          Loading summary...
        </div>
      )}
      {error && (
        <div className="bg-red-50 border border-red-200 text-red-700 rounded p-3 mb-4">Error loading summary. Please try again.</div>
      )}
      {!isLoading && !error && data && (
        <div className="grid grid-cols-1 md:grid-cols-3 gap-6 mb-8">
          <div className="bg-white rounded shadow p-6 flex flex-col items-center w-full">
            <span className="text-4xl font-bold text-brand-600">{data.total_drifts ?? 0}</span>
            <span className="text-gray-700 mt-2">Total Drifts</span>
          </div>
          <div className="bg-white rounded shadow p-6 flex flex-col items-center w-full">
            <span className="text-4xl font-bold text-green-600">{data.compliant_resources ?? 0}</span>
            <span className="text-gray-700 mt-2">Compliant Resources</span>
          </div>
          <div className="bg-white rounded shadow p-6 flex flex-col items-center w-full">
            <span className="text-4xl font-bold text-red-600">{data.noncompliant_resources ?? 0}</span>
            <span className="text-gray-700 mt-2">Noncompliant Resources</span>
          </div>
        </div>
      )}
      <p className="text-gray-700 text-center md:text-left">Welcome! This is your unified drift and compliance summary.</p>
      {/* Toast notifications placeholder */}
    </div>
  );
}
