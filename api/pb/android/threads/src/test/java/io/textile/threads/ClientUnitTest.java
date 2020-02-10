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

import java.util.Base64;
import com.google.gson.Gson;

import io.textile.threads_grpc.*;

import static org.junit.Assert.*;

@RunWith(JUnit4.class)
@FixMethodOrder(MethodSorters.NAME_ASCENDING)
public class ClientUnitTest {

    static Client client;
    static String storeId;
    static String modelId = "";

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
        String person = createPerson("", 22);
        String[] data = { person };
        ModelCreateReply reply = client.ModelCreateSync(storeId, "Person", data);
        assertEquals(1, reply.getEntitiesCount());
        String jsonString = reply.getEntities(0);
        Person model = new Gson().fromJson(jsonString, Person.class);
        modelId = model.ID;
        assertEquals(model.ID.length(), 36);
    }

    @Test
    public void t06_ModelSave() throws Exception {
        String person = createPerson(modelId, 22);
        String[] data = { person };
        client.ModelSaveSync(storeId, "Person", data);
        // now check that it's been updated
        ModelFindByIDReply reply = client.ModelFindByIDSync(storeId, "Person", modelId);
        String jsonString = reply.getEntity();
        Person model = new Gson().fromJson(jsonString, Person.class);
        assertEquals(modelId, model.ID);
    }

    private String createPerson(String ID, int age) throws Exception {
        Gson gson = new Gson();
        Person person = new Person(ID, age);
        String json = gson.toJson(person);
        return json;
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

