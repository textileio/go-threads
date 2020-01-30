package io.textile.threads;

import android.support.test.runner.AndroidJUnit4;

import org.junit.FixMethodOrder;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.MethodSorters;
import java.util.logging.Logger;
import static org.junit.Assert.assertEquals;

/**
 * Textile tests.
 */
@RunWith(AndroidJUnit4.class)
@FixMethodOrder(MethodSorters.NAME_ASCENDING)
public class ClientTest {

    private final static Logger logger =
            Logger.getLogger("TEST");

    static Client client;

    void connect() {
        // Start
        client = new Client("localhost", 6006);
        client.Connect();
    }

    @Test
    public void startTest() throws Exception {
        connect();
        assertEquals(true, client.connected());
    }

    @Test
    public void NewStore() throws Exception {
        if (client == null) {
            connect();
        }

        String storeId = client.NewStoreSync();

        assertEquals(36, storeId.length());
    }
}
