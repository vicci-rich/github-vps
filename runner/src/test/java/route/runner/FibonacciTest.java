package route.runner;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;

import org.junit.jupiter.api.Test;

class FibonacciTest {

    @Test
    void baseCases() {
        assertEquals(0L, Fibonacci.nth(0));
        assertEquals(1L, Fibonacci.nth(1));
        assertEquals(1L, Fibonacci.nth(2));
        assertEquals(55L, Fibonacci.nth(10));
    }

    @Test
    void sixtyTwoHops() {
        assertEquals(4052739537881L, Fibonacci.nth(62));
        long[] chain = Fibonacci.chain(62);
        assertEquals(63, chain.length);
        assertEquals(0L, chain[0]);
        assertEquals(1L, chain[1]);
        assertEquals(4052739537881L, chain[62]);
    }

    @Test
    void stepAdvancesThePair() {
        long a = 0;
        long b = 1;
        for (int i = 0; i < 62; i++) {
            long next = Fibonacci.step(a, b);
            a = b;
            b = next;
        }
        assertEquals(4052739537881L, a);
    }

    @Test
    void overflowBeyondTheSixtyFourthBitIsRejected() {
        assertEquals(7540113804746346429L, Fibonacci.nth(Fibonacci.MAX_SAFE_HOPS));
        assertThrows(IllegalArgumentException.class, () -> Fibonacci.nth(Fibonacci.MAX_SAFE_HOPS + 1));
        assertThrows(IllegalArgumentException.class, () -> Fibonacci.chain(0));
        assertThrows(IllegalArgumentException.class, () -> Fibonacci.chain(Fibonacci.MAX_SAFE_HOPS + 1));
        assertThrows(ArithmeticException.class, () -> Fibonacci.step(Long.MAX_VALUE, 1L));
    }
}
