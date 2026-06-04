import React from 'react';

const SEVERITY_STYLES = {
  critical: 'bg-red-100 text-red-700 border-red-200',
  high: 'bg-orange-100 text-orange-700 border-orange-200',
  medium: 'bg-yellow-100 text-yellow-700 border-yellow-200',
  low: 'bg-blue-100 text-blue-700 border-blue-200',
};

export function SeverityBadge({ severity }) {
  const cls = SEVERITY_STYLES[severity] || 'bg-gray-100 text-gray-700 border-gray-200';
  return (
    <span className={`inline-block px-2 py-0.5 rounded-full text-xs font-semibold border capitalize ${cls}`}>
      {severity || 'unknown'}
    </span>
  );
}

const DRIFT_STYLES = {
  modified: 'bg-purple-100 text-purple-700 border-purple-200',
  unmanaged: 'bg-gray-100 text-gray-600 border-gray-200',
  deleted: 'bg-red-100 text-red-700 border-red-200',
};

export function DriftTypeBadge({ type }) {
  const cls = DRIFT_STYLES[type] || 'bg-gray-100 text-gray-700 border-gray-200';
  return (
    <span className={`inline-block px-2 py-0.5 rounded-full text-xs font-medium border capitalize ${cls}`}>
      {type}
    </span>
  );
}

const PROVIDER_STYLES = {
  aws: 'bg-orange-50 text-orange-700 border-orange-200',
  gcp: 'bg-blue-50 text-blue-700 border-blue-200',
  azure: 'bg-sky-50 text-sky-700 border-sky-200',
};

export function ProviderBadge({ provider }) {
  const cls = PROVIDER_STYLES[provider] || 'bg-gray-50 text-gray-700 border-gray-200';
  return (
    <span className={`inline-block px-2 py-0.5 rounded text-xs font-semibold border uppercase ${cls}`}>
      {provider}
    </span>
  );
}
