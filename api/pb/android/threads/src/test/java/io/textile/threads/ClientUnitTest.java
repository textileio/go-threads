package io.textile.threads;

//import org.junit.FixMethodOrder;
//import org.junit.Test;
//import org.junit.runners.MethodSorters;
import org.json.JSONObject;
import org.junit.jupiter.api.MethodOrderer.OrderAnnotation;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestMethodOrder;
import org.junit.jupiter.api.Order;


import io.textile.threads_grpc.GetStoreLinkReply;
import io.textile.threads_grpc.ModelCreateReply;

import static org.junit.Assert.*;

@TestMethodOrder(OrderAnnotation.class)
public class ClientUnitTest {

    static Client client;
    static String storeId;

    void connect() throws Exception {
        // Initialize & start
        client = new Client("localhost", 6006);
        client.Connect();
    }

    @Test
    @Order(1)
    public void StartTest() throws Exception {
        connect();
        assertEquals(true, client.connected());
    }

    @Test
    @Order(2)
    public void NewStore() throws Exception {
        storeId = client.NewStoreSync();

        assertEquals(36, storeId.length());
    }

    @Test
    @Order(3)
    public void StartStore() throws Exception {
        client.StartSync(storeId);
        assertTrue(true);
    }

    @Test
    @Order(4)
    public void GetStoreLink() throws Exception {
        GetStoreLinkReply reply = client.GetStoreLinkSync(storeId);
        assertNotEquals(0, reply.getAddressesCount());
    }

    @Test
    @Order(4)
    public void RegisterSchema() throws Exception {
        String schema = "{\\r\\n  '\\\\$id': 'https:\\/\\/example.com\\/person.schema.json',\\r\\n  '\\\\$schema': 'http:\\/\\/json-schema.org\\/draft-07\\/schema#',\\r\\n  'title': 'Person',\\r\\n  'type': 'object',\\r\\n  'required': ['ID'],\\r\\n  'properties': {\\r\\n    'ID': {\\r\\n      'type': 'string',\\r\\n      'description': 'The entity\\\\'s id.',\\r\\n    },\\r\\n    'firstName': {\\r\\n      'type': 'string',\\r\\n      'description': 'The person\\\\'s first name.',\\r\\n    },\\r\\n    'lastName': {\\r\\n      'type': 'string',\\r\\n      'description': 'The person\\\\'s last name.',\\r\\n    },\\r\\n    'age': {\\r\\n      'description': 'Age in years which must be equal to or greater than zero.',\\r\\n      'type': 'integer',\\r\\n      'minimum': 0,\\r\\n    },\\r\\n  },\\r\\n};";
        client.RegisterSchemaSync(storeId, "Person", schema);
        assertTrue(true);
    }

    @Test
    @Order(5)
    public void ModelCreate() throws Exception {
        JSONObject person = createPerson("ABCDEF", 22);
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
}

