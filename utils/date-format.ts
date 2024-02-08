export function monthYear(date : Date) {
  return date.toLocaleString('en-US', { month: 'long', year: 'numeric'});
}