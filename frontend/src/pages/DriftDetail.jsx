import React, { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useParams, useNavigate } from 'react-router-dom';
import { ArrowLeft, Sparkles, GitPullRequest, CheckCircle2 } from 'lucide-react';
import axios from '../lib/axios';
import { SeverityBadge, DriftTypeBadge, ProviderBadge } from '../components/Badge';

function JsonBlock({ value }) {
  let text = value;
  try {
    text = JSON.stringify(typeof value === 'string' ? JSON.parse(value) : value, null, 2);
  } catch {
    /* leave as-is */
  }
  return (
    <pre className="bg-gray-900 text-gray-100 text-xs rounded-lg p-4 overflow-x-auto max-h-72">{text}</pre>
  );
}

export default function DriftDetail() {
  const { id } = useParams();
  const navigate = useNavigate();
  const qc = useQueryClient();
  const [pr, setPr] = useState({ repo_owner: '', repo_name: '', base_branch: 'main', file_path: '' });

  const { data, isLoading, error } = useQuery({
    queryKey: ['drift', id],
    queryFn: async () => (await axios.get(`/api/v1/drifts/${id}`)).data,
  });

  const generate = useMutation({
    mutationFn: async () => (await axios.post(`/api/v1/drifts/${id}/remediation`)).data,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['drift', id] }),
  });

  const resolve = useMutation({
    mutationFn: async () => (await axios.patch(`/api/v1/drifts/${id}/resolve`)).data,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['drift', id] });
      navigate('/drifts');
    },
  });

  const openPr = useMutation({
    mutationFn: async (remId) =>
      (await axios.post(`/api/v1/drifts/${id}/remediation/${remId}/pr`, pr)).data,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['drift', id] }),
  });

  if (isLoading) return <div className="p-8 text-gray-500 animate-pulse">Loading…</div>;
  if (error) return <div className="p-8 text-red-600">Drift not found.</div>;

  const diffFields = data.diff?.fields || [];
  const remediation = data.remediations?.[0];

  return (
    <div className="p-8 max-w-5xl mx-auto">
      <button onClick={() => navigate('/drifts')} className="flex items-center gap-1 text-gray-500 hover:text-brand-600 mb-4">
        <ArrowLeft className="w-4 h-4" /> Back to drifts
      </button>

      <div className="flex items-center gap-3 mb-2">
        <h1 className="text-2xl font-bold text-gray-800">
          {data.resource?.resource_name || data.resource?.resource_id}
        </h1>
        <SeverityBadge severity={data.severity} />
        <DriftTypeBadge type={data.drift_type} />
        <ProviderBadge provider={data.resource?.provider} />
      </div>
      <p className="text-gray-500 mb-6">
        {data.resource?.resource_type} · {data.resource?.region}
        {data.changed_by ? ` · changed by ${data.changed_by}` : ''}
      </p>

      {/* Compliance violations */}
      {data.violations?.length > 0 && (
        <div className="mb-6">
          <h2 className="text-lg font-semibold text-gray-700 mb-2">Compliance violations</h2>
          <div className="space-y-2">
            {data.violations.map((v) => (
              <div key={v.id} className="bg-white border border-gray-100 rounded-lg p-4 flex items-start gap-3 shadow-sm">
                <SeverityBadge severity={v.severity} />
                <div>
                  <div className="font-medium text-gray-800">
                    {v.policy_name} <span className="text-gray-400 text-xs uppercase">({v.framework} · {v.policy_id})</span>
                  </div>
                  <div className="text-sm text-gray-500">{v.description}</div>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Field diff */}
      {diffFields.length > 0 && (
        <div className="mb-6">
          <h2 className="text-lg font-semibold text-gray-700 mb-2">What changed</h2>
          <div className="bg-white border border-gray-100 rounded-lg overflow-hidden shadow-sm">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-left text-gray-500 bg-gray-50 border-b">
                  <th className="py-2 px-4">Field</th>
                  <th className="py-2 px-4">Declared</th>
                  <th className="py-2 px-4">Live</th>
                </tr>
              </thead>
              <tbody>
                {diffFields.map((f) => (
                  <tr key={f.field} className="border-b last:border-0">
                    <td className="py-2 px-4 font-mono text-gray-700">{f.field}</td>
                    <td className="py-2 px-4 font-mono text-green-700">{JSON.stringify(f.declared)}</td>
                    <td className="py-2 px-4 font-mono text-red-700">{JSON.stringify(f.live)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Live / declared state */}
      <div className="grid md:grid-cols-2 gap-4 mb-6">
        <div>
          <h3 className="text-sm font-semibold text-gray-600 mb-2">Live state</h3>
          <JsonBlock value={data.resource?.live_state} />
        </div>
        <div>
          <h3 className="text-sm font-semibold text-gray-600 mb-2">Declared state</h3>
          {data.resource?.declared_state ? (
            <JsonBlock value={data.resource.declared_state} />
          ) : (
            <div className="bg-gray-50 border border-dashed border-gray-300 rounded-lg p-4 text-sm text-gray-500 h-full">
              No declared state — this resource is not managed by connected IaC.
            </div>
          )}
        </div>
      </div>

      {/* AI remediation */}
      <div className="bg-white border border-gray-100 rounded-xl p-6 shadow-sm">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold text-gray-700 flex items-center gap-2">
            <Sparkles className="w-5 h-5 text-brand-600" /> AI Remediation
          </h2>
          {!data.is_resolved && (
            <button
              onClick={() => resolve.mutate()}
              className="flex items-center gap-1 text-sm text-green-600 hover:text-green-700"
            >
              <CheckCircle2 className="w-4 h-4" /> Mark resolved
            </button>
          )}
        </div>

        {!remediation && (
          <>
            <p className="text-sm text-gray-500 mb-3">
              Generate a Terraform patch that reconciles this drift, then open a PR to your IaC repo.
            </p>
            <button
              onClick={() => generate.mutate()}
              disabled={generate.isPending}
              className="flex items-center gap-2 bg-brand-600 text-white px-4 py-2 rounded-lg hover:bg-brand-700 transition disabled:opacity-50"
            >
              <Sparkles className="w-4 h-4" />
              {generate.isPending ? 'Generating…' : 'Generate fix'}
            </button>
            {generate.isError && (
              <div className="text-red-600 text-sm mt-3">
                {generate.error?.response?.data?.error || 'Failed to generate. Is an AI provider configured?'}
              </div>
            )}
          </>
        )}

        {remediation && (
          <>
            <JsonBlock value={remediation.patch} />
            {remediation.pr_url ? (
              <a
                href={remediation.pr_url}
                target="_blank"
                rel="noreferrer"
                className="inline-flex items-center gap-2 mt-4 bg-green-600 text-white px-4 py-2 rounded-lg hover:bg-green-700"
              >
                <GitPullRequest className="w-4 h-4" /> View PR #{remediation.pr_number}
              </a>
            ) : (
              <div className="mt-4 border-t pt-4">
                <h3 className="text-sm font-semibold text-gray-600 mb-2">Open a pull request</h3>
                <div className="grid grid-cols-2 gap-3 mb-3">
                  <input className="p-2 border rounded-lg text-sm" placeholder="repo owner"
                    value={pr.repo_owner} onChange={(e) => setPr({ ...pr, repo_owner: e.target.value })} />
                  <input className="p-2 border rounded-lg text-sm" placeholder="repo name"
                    value={pr.repo_name} onChange={(e) => setPr({ ...pr, repo_name: e.target.value })} />
                  <input className="p-2 border rounded-lg text-sm" placeholder="base branch (e.g. main)"
                    value={pr.base_branch} onChange={(e) => setPr({ ...pr, base_branch: e.target.value })} />
                  <input className="p-2 border rounded-lg text-sm" placeholder="file path (e.g. main.tf)"
                    value={pr.file_path} onChange={(e) => setPr({ ...pr, file_path: e.target.value })} />
                </div>
                <button
                  onClick={() => openPr.mutate(remediation.id)}
                  disabled={openPr.isPending || !pr.repo_owner || !pr.repo_name || !pr.file_path}
                  className="flex items-center gap-2 bg-gray-800 text-white px-4 py-2 rounded-lg hover:bg-black transition disabled:opacity-50"
                >
                  <GitPullRequest className="w-4 h-4" />
                  {openPr.isPending ? 'Opening PR…' : 'Open PR'}
                </button>
                {openPr.isError && (
                  <div className="text-red-600 text-sm mt-2">
                    {openPr.error?.response?.data?.error || 'Failed to open PR. Is GITHUB_TOKEN set?'}
                  </div>
                )}
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}
