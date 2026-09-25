interface CronField {
  min: number;
  max: number;
  values: Set<number>;
  wildcard: boolean;
}

const FIELD_RANGES: ReadonlyArray<{ min: number; max: number }> = [
  { min: 0, max: 59 },
  { min: 0, max: 23 },
  { min: 1, max: 31 },
  { min: 1, max: 12 },
  { min: 0, max: 6 },
];

function parseField(raw: string, range: { min: number; max: number }): CronField {
  const values = new Set<number>();
  const wildcard = raw === '*';
  for (const part of raw.split(',')) {
    const [rangePart, stepText] = part.split('/', 2);
    const step = stepText === undefined ? 1 : Number.parseInt(stepText, 10);
    if (!Number.isInteger(step) || step < 1) throw new Error(`invalid cron step: ${part}`);
    const bounds = rangePart === '*'
      ? [range.min, range.max]
      : rangePart.includes('-')
        ? rangePart.split('-', 2).map((value) => Number.parseInt(value, 10))
        : [Number.parseInt(rangePart, 10), Number.parseInt(rangePart, 10)];
    const [start, end] = bounds;
    if (!Number.isInteger(start) || !Number.isInteger(end) || start < range.min || end > range.max || start > end) {
      throw new Error(`invalid cron field: ${part}`);
    }
    for (let value = start; value <= end; value += step) values.add(value);
  }
  if (values.size === 0) throw new Error(`empty cron field: ${raw}`);
  return { ...range, values, wildcard };
}

function parseCron(expression: string): CronField[] {
  const parts = expression.trim().split(/\s+/);
  if (parts.length !== 5) throw new Error('cron must contain exactly five fields');
  return parts.map((part, index) => parseField(part, FIELD_RANGES[index]!));
}

function matchesDay(date: Date, dayOfMonth: CronField, dayOfWeek: CronField): boolean {
  const domMatches = dayOfMonth.values.has(date.getUTCDate());
  const dowMatches = dayOfWeek.values.has(date.getUTCDay());
  if (dayOfMonth.wildcard || dayOfWeek.wildcard) return domMatches && dowMatches;
  return domMatches || dowMatches;
}

/** Return the next UTC minute matching a standard five-field cron expression. */
export function nextCronFire(expression: string, from: Date): Date {
  const [minute, hour, dayOfMonth, month, dayOfWeek] = parseCron(expression);
  const cursor = new Date(from);
  cursor.setUTCSeconds(0, 0);
  cursor.setUTCMinutes(cursor.getUTCMinutes() + 1);
  const limit = cursor.getTime() + 366 * 24 * 60 * 60 * 1000;
  while (cursor.getTime() <= limit) {
    if (
      minute!.values.has(cursor.getUTCMinutes()) &&
      hour!.values.has(cursor.getUTCHours()) &&
      month!.values.has(cursor.getUTCMonth() + 1) &&
      matchesDay(cursor, dayOfMonth!, dayOfWeek!)
    ) return new Date(cursor);
    cursor.setUTCMinutes(cursor.getUTCMinutes() + 1);
  }
  throw new Error(`cron expression has no occurrence within one year: ${expression}`);
}
