package io.textile.threads;

import com.google.common.base.Charsets;
import com.google.common.io.Files;

import org.json.JSONObject;
import org.junit.Test;
import org.junit.FixMethodOrder;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;
import org.junit.runners.MethodSorters;


import java.io.BufferedReader;
import java.io.File;
import java.io.FileInputStream;
import java.io.InputStream;
import java.io.InputStreamReader;
import java.net.URL;

import io.textile.threads_grpc.GetStoreLinkReply;
import io.textile.threads_grpc.ModelCreateReply;

import static org.junit.Assert.*;

@RunWith(JUnit4.class)
@FixMethodOrder(MethodSorters.NAME_ASCENDING)
public class ClientUnitTest {

    static Client client;
    static String storeId;

    void connect() throws Exception {
        // Initialize & start
        client = new Client("localhost", 6006);
        client.Connect();
    }

    @Test
    public void t01_StartTest() throws Exception {
        connect();
        assertEquals(true, client.connected());
    }

    @Test
    public void t02_NewStore() throws Exception {
        storeId = client.NewStoreSync();

        assertEquals(36, storeId.length());
    }

    @Test
    public void t03_StartStore() throws Exception {
        client.StartSync(storeId);
        assertTrue(true);
    }

    @Test
    public void t04_GetStoreLink() throws Exception {
        GetStoreLinkReply reply = client.GetStoreLinkSync(storeId);
        assertNotEquals(0, reply.getAddressesCount());
    }

    @Test
    public void t05_RegisterSchema() throws Exception {
        String jsonStr = getStoredSchema();
        JSONObject json = new JSONObject(jsonStr);
        assertEquals(json.get("title").toString(), "Person");
        client.RegisterSchemaSync(storeId, "Person", jsonStr);
    }

    @Test
    public void t06_ModelCreate() throws Exception {
        JSONObject person = createPerson("", 22);
        String[] data = { person.toString() };
        ModelCreateReply reply = client.ModelCreateSync(storeId, "Person", data);
        assertEquals(1, reply.getEntitiesCount());
        // @todo once working, store modelid to update in later tests
    }

    @Test
    public void t06_ModelSave() throws Exception {
        // @todo use existing modelid
        JSONObject person = createPerson("", 22);
        String[] data = { person.toString() };
        ModelCreateReply reply = client.ModelCreateSync(storeId, "Person", data);
        assertEquals(1, reply.getEntitiesCount());
    }

    private JSONObject createPerson(String ID, int age) throws Exception {
        JSONObject obj = new JSONObject();
        obj.put("ID", ID);
        obj.put("firstName", "adam");
        obj.put("lastName", "doe");
        obj.put("age", new Integer(age));
        return obj;
    }

    private String getStoredSchema() throws Exception {
        BufferedReader br = new BufferedReader(new InputStreamReader(new FileInputStream("./src/test/resources/person.json")));
        StringBuilder sb = new StringBuilder();
        String line = br.readLine();
        while (line != null) {
            sb.append(line);
            line = br.readLine();
        }
        return sb.toString();
    }
}

