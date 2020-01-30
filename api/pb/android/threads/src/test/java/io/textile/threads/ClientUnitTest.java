package io.textile.threads;

import org.junit.Test;


import io.textile.threads_grpc.*;
import static org.junit.Assert.*;

public class ClientUnitTest {

    static Client client;

    void connect() throws Exception {
        // Initialize & start
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

        String storeId = client.NewStore();

        assertEquals(36, storeId.length());
    }
}
