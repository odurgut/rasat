/** Hash a service name onto the 6-stop ramp. Do not hand-pick colors. */
export function serviceRampIndex(name: string): number {
  let h = 0;
  for (let i = 0; i < name.length; i++) {
    h = (h * 31 + name.charCodeAt(i)) >>> 0;
  }
  return h % 6;
}
