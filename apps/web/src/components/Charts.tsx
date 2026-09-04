import { type KeyboardEvent, type PointerEvent, useId, useState } from "react";

import { t } from "../lib/i18n";

/**
 * Charts are drawn as plain SVG against the design tokens rather than through a charting
 * dependency. That keeps the bundle and the self-hosted image small, and it lets every mark
 * inherit the same palette, radii, and motion rules as the rest of the interface.
 *
 * Every chart is exposed twice: once as a picture for people who can see it, and once as a
 * table for people using a screen reader. The table is the accessible source of truth, so no
 * figure is available only as a shape.
 */

const TREND_WIDTH = 720;
const TREND_HEIGHT = 260;
const TREND_PADDING = { top: 16, right: 16, bottom: 30, left: 64 };

export type TrendSeries = {
  key: string;
  label: string;
  color: string;
  kind: "area" | "line";
  values: number[];
};

export function TrendChart({
  ariaLabel,
  formatValue,
  labels,
  series,
}: {
  ariaLabel: string;
  formatValue: (value: number) => string;
  labels: string[];
  series: TrendSeries[];
}) {
  const gradientId = useId();
  const [activeIndex, setActiveIndex] = useState<number | null>(null);
  const pointCount = labels.length;

  if (pointCount === 0 || series.length === 0) {
    return <p className="chart-empty">{t("No activity in this period.")}</p>;
  }

  const values = series.flatMap((entry) => entry.values);
  const maximum = Math.max(...values, 0);
  const minimum = Math.min(...values, 0);
  const span = maximum - minimum || 1;
  const plotWidth = TREND_WIDTH - TREND_PADDING.left - TREND_PADDING.right;
  const plotHeight = TREND_HEIGHT - TREND_PADDING.top - TREND_PADDING.bottom;
  // A single bucket has no horizontal span to divide, so it is centred instead of pinned to
  // the left edge where it would read as the start of a line that never continues.
  const x = (index: number) => TREND_PADDING.left + (pointCount === 1
    ? plotWidth / 2
    : (index / (pointCount - 1)) * plotWidth);
  const y = (value: number) =>
    TREND_PADDING.top + plotHeight - ((value - minimum) / span) * plotHeight;
  const ticks = [maximum, minimum + span / 2, minimum].filter(
    (value, index, all) => all.indexOf(value) === index,
  );
  const labelStep = Math.max(1, Math.ceil(pointCount / 7));

  function pointerIndex(event: PointerEvent<SVGSVGElement>): number {
    const bounds = event.currentTarget.getBoundingClientRect();
    if (bounds.width === 0) return 0;
    const position = ((event.clientX - bounds.left) / bounds.width) * TREND_WIDTH;
    const ratio = (position - TREND_PADDING.left) / plotWidth;
    return Math.min(pointCount - 1, Math.max(0, Math.round(ratio * (pointCount - 1))));
  }

  function handleKeyDown(event: KeyboardEvent<SVGSVGElement>) {
    if (event.key !== "ArrowLeft" && event.key !== "ArrowRight") return;
    event.preventDefault();
    const step = event.key === "ArrowRight" ? 1 : -1;
    setActiveIndex((current) => {
      const next = (current ?? (step > 0 ? -1 : pointCount)) + step;
      return Math.min(pointCount - 1, Math.max(0, next));
    });
  }

  return (
    <div className="chart-frame">
      <svg
        aria-label={ariaLabel}
        className="trend-chart"
        onBlur={() => setActiveIndex(null)}
        onKeyDown={handleKeyDown}
        onPointerLeave={() => setActiveIndex(null)}
        onPointerMove={(event) => setActiveIndex(pointerIndex(event))}
        role="img"
        tabIndex={0}
        viewBox={`0 0 ${TREND_WIDTH} ${TREND_HEIGHT}`}
      >
        <defs>
          {series.filter((entry) => entry.kind === "area").map((entry) => (
            <linearGradient id={`${gradientId}-${entry.key}`} key={entry.key} x1="0" x2="0" y1="0" y2="1">
              <stop offset="0%" stopColor={entry.color} stopOpacity="0.32" />
              <stop offset="100%" stopColor={entry.color} stopOpacity="0.02" />
            </linearGradient>
          ))}
        </defs>
        {ticks.map((tick) => (
          <g key={tick}>
            <line
              className="chart-gridline"
              x1={TREND_PADDING.left}
              x2={TREND_WIDTH - TREND_PADDING.right}
              y1={y(tick)}
              y2={y(tick)}
            />
            <text className="chart-axis-label" textAnchor="end" x={TREND_PADDING.left - 10} y={y(tick) + 4}>
              {formatValue(tick)}
            </text>
          </g>
        ))}
        {minimum < 0 ? (
          <line
            className="chart-zero-line"
            x1={TREND_PADDING.left}
            x2={TREND_WIDTH - TREND_PADDING.right}
            y1={y(0)}
            y2={y(0)}
          />
        ) : null}
        {series.map((entry) => {
          const line = entry.values.map((value, index) => `${index === 0 ? "M" : "L"}${x(index)} ${y(value)}`).join(" ");
          return (
            <g key={entry.key}>
              {entry.kind === "area" ? (
                <path
                  d={`${line} L${x(pointCount - 1)} ${y(minimum)} L${x(0)} ${y(minimum)} Z`}
                  fill={`url(#${gradientId}-${entry.key})`}
                />
              ) : null}
              <path
                className={`chart-line chart-line-${entry.kind}`}
                d={line}
                stroke={entry.color}
              />
              {pointCount === 1 ? (
                <circle cx={x(0)} cy={y(entry.values[0])} fill={entry.color} r="4" />
              ) : null}
            </g>
          );
        })}
        {activeIndex !== null ? (
          <g>
            <line
              className="chart-crosshair"
              x1={x(activeIndex)}
              x2={x(activeIndex)}
              y1={TREND_PADDING.top}
              y2={TREND_PADDING.top + plotHeight}
            />
            {series.map((entry) => (
              <circle
                cx={x(activeIndex)}
                cy={y(entry.values[activeIndex])}
                fill={entry.color}
                key={entry.key}
                r="4.5"
                stroke="var(--surface)"
                strokeWidth="2"
              />
            ))}
          </g>
        ) : null}
        {labels.map((label, index) => (
          index % labelStep === 0 || index === pointCount - 1 ? (
            <text
              className="chart-axis-label"
              key={label + index}
              textAnchor="middle"
              x={x(index)}
              y={TREND_HEIGHT - 8}
            >
              {label}
            </text>
          ) : null
        ))}
      </svg>
      {activeIndex !== null ? (
        // Near either edge the tooltip is anchored by its own side instead of its centre, so
        // it never spills out of the card it belongs to.
        <div
          className="chart-tooltip"
          data-align={tooltipAlignment(x(activeIndex) / TREND_WIDTH)}
          style={{ insetInlineStart: `${(x(activeIndex) / TREND_WIDTH) * 100}%` }}
        >
          <strong>{labels[activeIndex]}</strong>
          {series.map((entry) => (
            <span key={entry.key}>
              <i aria-hidden="true" style={{ background: entry.color }} />
              {entry.label}
              <b>{formatValue(entry.values[activeIndex])}</b>
            </span>
          ))}
        </div>
      ) : null}
      <ChartDataTable
        caption={ariaLabel}
        columns={series.map((entry) => entry.label)}
        rows={labels.map((label, index) => ({
          header: label,
          cells: series.map((entry) => formatValue(entry.values[index])),
        }))}
      />
    </div>
  );
}

function tooltipAlignment(ratio: number): "center" | "end" | "start" {
  if (ratio > 0.72) return "end";
  if (ratio < 0.28) return "start";
  return "center";
}

export type DonutSlice = { id: string; label: string; value: number; color: string };

export function DonutChart({
  ariaLabel,
  centerLabel,
  centerValue,
  formatValue,
  slices,
}: {
  ariaLabel: string;
  centerLabel: string;
  centerValue: string;
  formatValue: (value: number) => string;
  slices: DonutSlice[];
}) {
  const total = slices.reduce((sum, slice) => sum + Math.max(slice.value, 0), 0);
  if (total <= 0) {
    return <p className="chart-empty">{t("No activity in this period.")}</p>;
  }
  const radius = 62;
  const circumference = 2 * Math.PI * radius;
  let offset = 0;

  return (
    <div className="donut-frame">
      <svg aria-label={ariaLabel} className="donut-chart" role="img" viewBox="0 0 160 160">
        {/* Only the ring is rotated so the first slice starts at twelve o'clock. Rotating the
            whole drawing would take the centred text with it. */}
        <g transform="rotate(-90 80 80)">
          <circle className="donut-track" cx="80" cy="80" fill="none" r={radius} />
          {slices.map((slice) => {
            const share = Math.max(slice.value, 0) / total;
            const dash = share * circumference;
            const element = (
              <circle
                className="donut-slice"
                cx="80"
                cy="80"
                fill="none"
                key={slice.id}
                r={radius}
                stroke={slice.color}
                strokeDasharray={`${dash} ${circumference - dash}`}
                strokeDashoffset={-offset}
              />
            );
            offset += dash;
            return element;
          })}
        </g>
        <text className="donut-center-value" textAnchor="middle" x="80" y="78">{centerValue}</text>
        <text className="donut-center-label" textAnchor="middle" x="80" y="96">{centerLabel}</text>
      </svg>
      <ChartDataTable
        caption={ariaLabel}
        columns={[t("Amount"), t("Share")]}
        rows={slices.map((slice) => ({
          header: slice.label,
          cells: [
            formatValue(slice.value),
            new Intl.NumberFormat(undefined, { style: "percent", maximumFractionDigits: 1 })
              .format(Math.max(slice.value, 0) / total),
          ],
        }))}
      />
    </div>
  );
}

/** A single proportional bar. The width is the reading; the label carries the exact figure. */
export function MeterBar({
  color,
  label,
  share,
}: {
  color?: string;
  label: string;
  share: number;
}) {
  return (
    <span aria-label={label} className="meter-bar" role="img">
      <span
        style={{
          background: color ?? "var(--chart-spending)",
          inlineSize: `${Math.max(0, Math.min(1, share)) * 100}%`,
        }}
      />
    </span>
  );
}

export function Sparkline({
  ariaLabel,
  color,
  values,
}: {
  ariaLabel: string;
  color: string;
  values: number[];
}) {
  if (values.length < 2) return null;
  const maximum = Math.max(...values, 0);
  const minimum = Math.min(...values, 0);
  const span = maximum - minimum || 1;
  const path = values
    .map((value, index) => {
      const x = (index / (values.length - 1)) * 100;
      const y = 24 - ((value - minimum) / span) * 22 - 1;
      return `${index === 0 ? "M" : "L"}${x.toFixed(2)} ${y.toFixed(2)}`;
    })
    .join(" ");
  return (
    <svg aria-label={ariaLabel} className="sparkline" role="img" viewBox="0 0 100 24">
      <path d={path} stroke={color} />
    </svg>
  );
}

export type HeatmapCell = {
  date: string;
  intensity: number;
  label: string;
};

export function HeatmapGrid({
  ariaLabel,
  columns,
  weekdayLabels,
}: {
  ariaLabel: string;
  columns: { key: string; cells: (HeatmapCell | null)[] }[];
  weekdayLabels: string[];
}) {
  return (
    <div aria-label={ariaLabel} className="heatmap" role="img">
      <div className="heatmap-weekdays" aria-hidden="true">
        {/* Narrow weekday initials repeat within a week, so the position is the identity. */}
        {weekdayLabels.map((label, index) => <span key={index}>{label}</span>)}
      </div>
      <div className="heatmap-columns">
        {columns.map((column) => (
          <div className="heatmap-column" key={column.key}>
            {column.cells.map((cell, index) => (
              cell ? (
                <span
                  className="heatmap-cell"
                  key={cell.date}
                  // The opacity floor keeps a day with activity visible against an empty one,
                  // which a linear scale would otherwise render as indistinguishable.
                  style={{ opacity: cell.intensity > 0 ? 0.18 + cell.intensity * 0.82 : 1 }}
                  data-empty={cell.intensity === 0 ? "true" : undefined}
                >
                  <span className="visually-hidden">{cell.label}</span>
                </span>
              ) : (
                <span className="heatmap-cell heatmap-cell-outside" key={`${column.key}-${index}`} />
              )
            ))}
          </div>
        ))}
      </div>
    </div>
  );
}

/**
 * The tabular twin of a chart. It is hidden from sight but not from assistive technology,
 * which is what keeps the visual and the accessible readings from drifting apart.
 */
export function ChartDataTable({
  caption,
  columns,
  rows,
}: {
  caption: string;
  columns: string[];
  rows: { header: string; cells: string[] }[];
}) {
  return (
    <table className="visually-hidden">
      <caption>{caption}</caption>
      <thead>
        <tr>
          <th scope="col">{t("Period")}</th>
          {columns.map((column) => <th key={column} scope="col">{column}</th>)}
        </tr>
      </thead>
      <tbody>
        {/* Row headers can repeat across a multi-year window, so the position is the key. */}
        {rows.map((row, index) => (
          <tr key={index}>
            <th scope="row">{row.header}</th>
            {row.cells.map((cell, index) => <td key={columns[index] ?? index}>{cell}</td>)}
          </tr>
        ))}
      </tbody>
    </table>
  );
}
