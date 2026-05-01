"use client";

import { useEffect, useState } from "react";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

interface Stats {
  pending: number;
  processing: number;
  completed: number;
  dead: number;
  total: number;
}

interface Job {
  ID: string;
  Type: string;
  Status: string;
  Payload: string;
  Attempts: number;
  MaxRetries: number;
  LastError: string;
  CreatedAt: number;
  UpdatedAt: number;
}

export default function Dashboard() {
  const [stats, setStats] = useState<Stats | null>(null);
  const [deadJobs, setDeadJobs] = useState<Job[]>([]);
  const [loading, setLoading] = useState(true);
  const [enqueueing, setEnqueueing] = useState(false);
  const [jobType, setJobType] = useState("email");
  const [jobCount, setJobCount] = useState(5);
  const [shouldFail, setShouldFail] = useState(false);

  const fetchData = async () => {
    try {
      const [statsRes, dlqRes] = await Promise.all([
        fetch(`${API_URL}/stats`),
        fetch(`${API_URL}/dlq`),
      ]);
      const statsData = await statsRes.json();
      const dlqData = await dlqRes.json();
      setStats(statsData);
      setDeadJobs(dlqData || []);
    } catch (error) {
      console.error("Failed to fetch data:", error);
    } finally {
      setLoading(false);
    }
  };

  const retryJob = async (id: string) => {
    try {
      await fetch(`${API_URL}/dlq/${id}/retry`, { method: "POST" });
      fetchData();
    } catch (error) {
      console.error("Failed to retry job:", error);
    }
  };

  const retryAllDead = async () => {
    for (const job of deadJobs) {
      await fetch(`${API_URL}/dlq/${job.ID}/retry`, { method: "POST" });
    }
    fetchData();
  };

  const enqueueJobs = async () => {
    setEnqueueing(true);
    try {
      const promises = Array.from({ length: jobCount }, (_, i) =>
        fetch(`${API_URL}/jobs`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            type: jobType,
            payload: {
              id: i + 1,
              shouldFail,
              timestamp: Date.now(),
              message: `Test job ${i + 1}`,
            },
            priority: Math.floor(Math.random() * 20) - 10,
          }),
        })
      );
      await Promise.all(promises);
      fetchData();
    } catch (error) {
      console.error("Failed to enqueue jobs:", error);
    } finally {
      setEnqueueing(false);
    }
  };

  useEffect(() => {
    fetchData();
    const interval = setInterval(fetchData, 1000);
    return () => clearInterval(interval);
  }, []);

  if (loading) {
    return (
      <div className="min-h-screen bg-[#0a0a0a] flex items-center justify-center">
        <div className="flex items-center gap-3">
          <div className="w-5 h-5 border-2 border-blue-500 border-t-transparent rounded-full animate-spin" />
          <span className="text-zinc-400">Loading...</span>
        </div>
      </div>
    );
  }

  const successRate = stats && stats.total > 0
    ? Math.round((stats.completed / (stats.completed + stats.dead)) * 100) || 100
    : 100;

  return (
    <div className="min-h-screen bg-[#0a0a0a] text-zinc-100">
      {/* Header */}
      <header className="border-b border-zinc-800/50 bg-[#0a0a0a]/80 backdrop-blur-sm sticky top-0 z-50">
        <div className="max-w-7xl mx-auto px-6 py-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className="w-10 h-10 rounded-xl bg-gradient-to-br from-blue-500 to-purple-600 flex items-center justify-center">
                <svg className="w-5 h-5 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
                </svg>
              </div>
              <div>
                <h1 className="text-xl font-semibold text-white">Forge</h1>
                <p className="text-xs text-zinc-500">Real-time monitoring</p>
              </div>
            </div>
            <div className="flex items-center gap-2">
              <div className="w-2 h-2 rounded-full bg-green-500 animate-pulse" />
              <span className="text-sm text-zinc-400">Live</span>
            </div>
          </div>
        </div>
      </header>

      <main className="max-w-7xl mx-auto px-6 py-8">
        {/* Stats Grid */}
        <div className="grid grid-cols-2 lg:grid-cols-5 gap-4 mb-8">
          <StatCard
            title="Pending"
            value={stats?.pending || 0}
            icon="⏳"
            color="from-amber-500/20 to-amber-600/5"
            borderColor="border-amber-500/20"
            textColor="text-amber-400"
          />
          <StatCard
            title="Processing"
            value={stats?.processing || 0}
            icon="⚡"
            color="from-blue-500/20 to-blue-600/5"
            borderColor="border-blue-500/20"
            textColor="text-blue-400"
          />
          <StatCard
            title="Completed"
            value={stats?.completed || 0}
            icon="✓"
            color="from-emerald-500/20 to-emerald-600/5"
            borderColor="border-emerald-500/20"
            textColor="text-emerald-400"
          />
          <StatCard
            title="Failed"
            value={stats?.dead || 0}
            icon="✕"
            color="from-red-500/20 to-red-600/5"
            borderColor="border-red-500/20"
            textColor="text-red-400"
          />
          <StatCard
            title="Total"
            value={stats?.total || 0}
            icon="∑"
            color="from-zinc-500/20 to-zinc-600/5"
            borderColor="border-zinc-500/20"
            textColor="text-zinc-400"
          />
        </div>

        <div className="grid lg:grid-cols-3 gap-6 mb-8">
          {/* Enqueue Panel */}
          <div className="lg:col-span-1 bg-zinc-900/50 rounded-2xl p-6 border border-zinc-800/50">
            <h2 className="text-lg font-semibold mb-4 flex items-center gap-2">
              <span className="text-xl">🚀</span>
              Queue Jobs
            </h2>

            <div className="space-y-4">
              <div>
                <label className="text-sm text-zinc-400 mb-2 block">Job Type</label>
                <div className="grid grid-cols-3 gap-2">
                  {["email", "webhook", "report"].map((type) => (
                    <button
                      key={type}
                      onClick={() => setJobType(type)}
                      className={`px-3 py-2 rounded-lg text-sm font-medium transition-all ${
                        jobType === type
                          ? "bg-blue-600 text-white"
                          : "bg-zinc-800 text-zinc-400 hover:bg-zinc-700"
                      }`}
                    >
                      {type}
                    </button>
                  ))}
                </div>
              </div>

              <div>
                <label className="text-sm text-zinc-400 mb-2 block">Number of Jobs</label>
                <div className="flex gap-2">
                  {[1, 5, 10, 25, 50].map((n) => (
                    <button
                      key={n}
                      onClick={() => setJobCount(n)}
                      className={`flex-1 px-2 py-2 rounded-lg text-sm font-medium transition-all ${
                        jobCount === n
                          ? "bg-purple-600 text-white"
                          : "bg-zinc-800 text-zinc-400 hover:bg-zinc-700"
                      }`}
                    >
                      {n}
                    </button>
                  ))}
                </div>
              </div>

              <div>
                <label className="flex items-center gap-3 cursor-pointer">
                  <div
                    onClick={() => setShouldFail(!shouldFail)}
                    className={`w-12 h-6 rounded-full transition-all ${
                      shouldFail ? "bg-red-600" : "bg-zinc-700"
                    } relative`}
                  >
                    <div
                      className={`w-5 h-5 rounded-full bg-white absolute top-0.5 transition-all ${
                        shouldFail ? "left-6" : "left-0.5"
                      }`}
                    />
                  </div>
                  <span className="text-sm text-zinc-400">Simulate failures</span>
                </label>
              </div>

              <button
                onClick={enqueueJobs}
                disabled={enqueueing}
                className="w-full py-3 bg-gradient-to-r from-blue-600 to-purple-600 hover:from-blue-500 hover:to-purple-500 disabled:opacity-50 rounded-xl font-semibold transition-all flex items-center justify-center gap-2"
              >
                {enqueueing ? (
                  <>
                    <div className="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin" />
                    Queueing...
                  </>
                ) : (
                  <>
                    <span>Queue {jobCount} Jobs</span>
                    <span>→</span>
                  </>
                )}
              </button>
            </div>
          </div>

          {/* Health Panel */}
          <div className="lg:col-span-2 bg-zinc-900/50 rounded-2xl p-6 border border-zinc-800/50">
            <h2 className="text-lg font-semibold mb-4 flex items-center gap-2">
              <span className="text-xl">📊</span>
              Queue Health
            </h2>

            {/* Success Rate */}
            <div className="mb-6">
              <div className="flex justify-between items-center mb-2">
                <span className="text-sm text-zinc-400">Success Rate</span>
                <span className={`text-2xl font-bold ${
                  successRate >= 90 ? "text-emerald-400" :
                  successRate >= 70 ? "text-amber-400" : "text-red-400"
                }`}>
                  {successRate}%
                </span>
              </div>
              <div className="h-3 bg-zinc-800 rounded-full overflow-hidden">
                <div
                  className={`h-full transition-all duration-500 rounded-full ${
                    successRate >= 90 ? "bg-gradient-to-r from-emerald-500 to-emerald-400" :
                    successRate >= 70 ? "bg-gradient-to-r from-amber-500 to-amber-400" :
                    "bg-gradient-to-r from-red-500 to-red-400"
                  }`}
                  style={{ width: `${successRate}%` }}
                />
              </div>
            </div>

            {/* Distribution Bar */}
            <div className="mb-4">
              <div className="flex justify-between items-center mb-2">
                <span className="text-sm text-zinc-400">Job Distribution</span>
              </div>
              <div className="h-8 bg-zinc-800 rounded-xl overflow-hidden flex">
                {stats && stats.total > 0 ? (
                  <>
                    <div
                      className="bg-gradient-to-b from-emerald-400 to-emerald-600 transition-all duration-500 flex items-center justify-center text-xs font-medium"
                      style={{ width: `${(stats.completed / stats.total) * 100}%` }}
                    >
                      {stats.completed > 0 && stats.completed}
                    </div>
                    <div
                      className="bg-gradient-to-b from-blue-400 to-blue-600 transition-all duration-500 flex items-center justify-center text-xs font-medium"
                      style={{ width: `${(stats.processing / stats.total) * 100}%` }}
                    >
                      {stats.processing > 0 && stats.processing}
                    </div>
                    <div
                      className="bg-gradient-to-b from-amber-400 to-amber-600 transition-all duration-500 flex items-center justify-center text-xs font-medium"
                      style={{ width: `${(stats.pending / stats.total) * 100}%` }}
                    >
                      {stats.pending > 0 && stats.pending}
                    </div>
                    <div
                      className="bg-gradient-to-b from-red-400 to-red-600 transition-all duration-500 flex items-center justify-center text-xs font-medium"
                      style={{ width: `${(stats.dead / stats.total) * 100}%` }}
                    >
                      {stats.dead > 0 && stats.dead}
                    </div>
                  </>
                ) : (
                  <div className="flex-1 flex items-center justify-center text-zinc-500 text-sm">
                    No jobs yet
                  </div>
                )}
              </div>
            </div>

            {/* Legend */}
            <div className="flex flex-wrap gap-4 text-sm">
              <span className="flex items-center gap-2">
                <span className="w-3 h-3 rounded-full bg-emerald-500" /> Completed
              </span>
              <span className="flex items-center gap-2">
                <span className="w-3 h-3 rounded-full bg-blue-500" /> Processing
              </span>
              <span className="flex items-center gap-2">
                <span className="w-3 h-3 rounded-full bg-amber-500" /> Pending
              </span>
              <span className="flex items-center gap-2">
                <span className="w-3 h-3 rounded-full bg-red-500" /> Failed
              </span>
            </div>
          </div>
        </div>

        {/* Dead Letter Queue */}
        <div className="bg-zinc-900/50 rounded-2xl p-6 border border-zinc-800/50">
          <div className="flex justify-between items-center mb-4">
            <h2 className="text-lg font-semibold flex items-center gap-2">
              <span className="text-xl">💀</span>
              Dead Letter Queue
            </h2>
            {deadJobs.length > 0 && (
              <button
                onClick={retryAllDead}
                className="px-4 py-2 bg-zinc-800 hover:bg-zinc-700 rounded-lg text-sm font-medium transition-all flex items-center gap-2"
              >
                <span>↻</span>
                Retry All ({deadJobs.length})
              </button>
            )}
          </div>

          {deadJobs.length === 0 ? (
            <div className="text-center py-12">
              <div className="text-4xl mb-3">🎉</div>
              <div className="text-zinc-400">No failed jobs!</div>
              <div className="text-zinc-600 text-sm">All systems operational</div>
            </div>
          ) : (
            <div className="space-y-2 max-h-80 overflow-y-auto">
              {deadJobs.map((job) => (
                <div
                  key={job.ID}
                  className="bg-zinc-800/50 hover:bg-zinc-800 rounded-xl p-4 flex items-center justify-between transition-all group"
                >
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 mb-1">
                      <code className="text-xs text-zinc-500 font-mono">
                        {job.ID.slice(0, 8)}
                      </code>
                      <span className="px-2 py-0.5 bg-zinc-700 rounded text-xs font-medium">
                        {job.Type}
                      </span>
                      <span className="px-2 py-0.5 bg-red-900/50 text-red-400 rounded text-xs">
                        {job.Attempts}/{job.MaxRetries} attempts
                      </span>
                    </div>
                    <div className="text-sm text-red-400 truncate">
                      {job.LastError || "Unknown error"}
                    </div>
                  </div>
                  <button
                    onClick={() => retryJob(job.ID)}
                    className="ml-4 px-4 py-2 bg-blue-600 hover:bg-blue-500 rounded-lg text-sm font-medium transition-all opacity-70 group-hover:opacity-100"
                  >
                    Retry
                  </button>
                </div>
              ))}
            </div>
          )}
        </div>
      </main>
    </div>
  );
}

function StatCard({
  title,
  value,
  icon,
  color,
  borderColor,
  textColor,
}: {
  title: string;
  value: number;
  icon: string;
  color: string;
  borderColor: string;
  textColor: string;
}) {
  return (
    <div className={`bg-gradient-to-br ${color} rounded-2xl p-5 border ${borderColor} transition-all hover:scale-[1.02]`}>
      <div className="flex items-center justify-between mb-3">
        <span className="text-2xl">{icon}</span>
        <span className={`text-xs font-medium ${textColor}`}>{title}</span>
      </div>
      <div className="text-3xl font-bold text-white">{value.toLocaleString()}</div>
    </div>
  );
}
