package io.textile.threads;

import org.json.JSONObject;
import org.junit.Test;
import org.junit.FixMethodOrder;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;
import org.junit.runners.MethodSorters;


import java.io.BufferedReader;
import java.io.FileInputStream;
import java.io.InputStreamReader;
import java.nio.charset.StandardCharsets;

import com.google.common.io.BaseEncoding;
import com.google.gson.Gson;
import com.google.protobuf.ByteString;

import io.textile.threads_grpc.*;

import static org.junit.Assert.*;

@RunWith(JUnit4.class)
@FixMethodOrder(MethodSorters.NAME_ASCENDING)
public class ClientUnitTest {

    static Client client;
    static String dbId = "AVXwYdq9KAKa/qBCJulxduX3IuaiRjB6R68=";
    static String instanceId = "";

    void connect() throws Exception {
        // Initialize & start
        client = new Client();
        // Await initialization
        client.init().get();
        System.out.println("Thread Info: " + "READY!");
    }

    @Test
    public void t01_StartTest() throws Exception {
        connect();
        assertEquals(true, client.connected());
    }

    @Test
    public void t02_NewDB() throws Exception {
        Credentials.Builder creds = Credentials.newBuilder();
        creds.setThreadID(ByteString.copyFrom(BaseEncoding.base64().decode(dbId)));
        client.NewDBSync(creds.build());
    }

    @Test
    public void t04_GetDBInfo() throws Exception {
        Credentials.Builder creds = Credentials.newBuilder();
        creds.setThreadID(ByteString.copyFrom(BaseEncoding.base64().decode(dbId)));
        GetDBInfoReply reply = client.GetDBInfoSync(creds.build());
        assertNotEquals(0, reply.getAddressesCount());
    }

    @Test
    public void t05_NewCollection() throws Exception {
        String jsonStr = getStoredSchema();
        ByteString schema = ByteString.copyFrom(jsonStr.getBytes());
        JSONObject json = new JSONObject(jsonStr);
        assertEquals(json.get("title").toString(), "Person");
        Credentials.Builder creds = Credentials.newBuilder();
        creds.setThreadID(ByteString.copyFrom(BaseEncoding.base64().decode(dbId)));
        client.NewCollectionSync(creds.build(), "Person", schema);
    }

    @Test
    public void t06_Create() throws Exception {
        ByteString person = createPerson("", 22);
        ByteString[] data = { person };
        Credentials.Builder creds = Credentials.newBuilder();
        creds.setThreadID(ByteString.copyFrom(BaseEncoding.base64().decode(dbId)));
        CreateReply reply = client.CreateSync(creds.build(), "Person", data);
        assertEquals(1, reply.getInstanceIDsCount());
        String id = reply.getInstanceIDs(0);
        instanceId = id;
        assertEquals(id.length(), 36);
    }

    @Test
    public void t06_Save() throws Exception {
        ByteString person = createPerson(instanceId, 22);
        ByteString[] data = { person };
        Credentials.Builder creds = Credentials.newBuilder();
        creds.setThreadID(ByteString.copyFrom(BaseEncoding.base64().decode(dbId)));
        client.SaveSync(creds.build(), "Person", data);
        // now check that it's been updated
        FindByIDReply reply = client.FindByIDSync(creds.build(), "Person", instanceId);
        ByteString jsonBytes = reply.getInstance();
        Person instance = new Gson().fromJson(jsonBytes.toStringUtf8(), Person.class);
        assertEquals(instanceId, instance.ID);
    }

    private ByteString createPerson(String ID, int age) throws Exception {
        Gson gson = new Gson();
        Person person = new Person(ID, age);
        String json = gson.toJson(person);
        return ByteString.copyFrom(json.getBytes());
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

class Person {
    public String firstName = "adam";
    public String lastName = "doe";
    public String ID;
    public int age;
    Person(String ID, int age) {
        this.age = age;
        this.ID = ID;
    }
}

