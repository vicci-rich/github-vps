package route.runner;

/**
 * Iterative Fibonacci over 64-bit integers. The 62-hop linkage chain in the
 * plan stays inside the signed range: F(92) = 7540113804746346429 fits, F(93)
 * overflows, so {@link #MAX_SAFE_HOPS} is 92 and the default chain is 62 hops
 * (F(62) = 4052739537881656763).
 */
public final class Fibonacci {

    /** Highest hop count that still fits a signed 64-bit integer. */
    public static final int MAX_SAFE_HOPS = 92;

    private Fibonacci() {
    }

    /** One delegatecall hop: returns a + b, failing on overflow. */
    public static long step(long a, long b) {
        return Math.addExact(a, b);
    }

    /** F(n) with F(0) = 0, F(1) = 1. */
    public static long nth(int n) {
        if (n < 0 || n > MAX_SAFE_HOPS) {
            throw new IllegalArgumentException("n out of range: " + n);
        }
        if (n == 0) {
            return 0L;
        }
        long a = 0L;
        long b = 1L;
        for (int i = 2; i <= n; i++) {
            long next = step(a, b);
            a = b;
            b = next;
        }
        return b;
    }

    /** The whole chain F(0)..F(hops), one entry per state. */
    public static long[] chain(int hops) {
        if (hops < 1 || hops > MAX_SAFE_HOPS) {
            throw new IllegalArgumentException("hops out of range: " + hops);
        }
        long[] out = new long[hops + 1];
        out[0] = 0L;
        out[1] = 1L;
        for (int i = 2; i <= hops; i++) {
            out[i] = step(out[i - 2], out[i - 1]);
        }
        return out;
    }
}
