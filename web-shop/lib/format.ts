export function paiseToINR(paise: number): string {
  // Show paise precision (2 decimals) when the amount isn't a whole rupee so
  // customers see the exact charge that Razorpay will collect.
  const hasFraction = paise % 100 !== 0;
  return `₹${(paise / 100).toLocaleString("en-IN", {
    minimumFractionDigits: hasFraction ? 2 : 0,
    maximumFractionDigits: 2,
  })}`;
}

export const formatPaise = paiseToINR;
