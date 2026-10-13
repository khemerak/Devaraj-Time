import {
  AreaChart,
  Area,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  CartesianGrid,
} from 'recharts';
export default function TrendChart({ data }: { data: any[] }) {
  return (
    <ResponsiveContainer width="100%" height={245}>
      <AreaChart data={data} margin={{ top: 15, right: 10, bottom: 0, left: -25 }}>
        <defs>
          <linearGradient id="fillBlue" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor="#2563eb" stopOpacity={0.18} />
            <stop offset="100%" stopColor="#2563eb" stopOpacity={0} />
          </linearGradient>
        </defs>
        <CartesianGrid strokeDasharray="3 5" vertical={false} stroke="var(--border)" />
        <XAxis dataKey="date" tickFormatter={(v) => v.slice(5)} axisLine={false} tickLine={false} />
        <YAxis allowDecimals={false} axisLine={false} tickLine={false} />
        <Tooltip />
        <Area
          name="Recorded sessions"
          type="monotone"
          dataKey="present"
          stroke="#2563eb"
          strokeWidth={3}
          fill="url(#fillBlue)"
        />
        <Area
          name="Late sessions"
          type="monotone"
          dataKey="late"
          stroke="#d97706"
          fill="transparent"
          strokeWidth={2}
        />
      </AreaChart>
    </ResponsiveContainer>
  );
}
