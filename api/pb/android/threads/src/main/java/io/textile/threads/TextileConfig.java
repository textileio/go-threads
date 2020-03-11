package io.textile.threads;

import com.google.gson.Gson;
import com.google.gson.JsonObject;
import com.squareup.okhttp.MediaType;
import com.squareup.okhttp.OkHttpClient;
import com.squareup.okhttp.Protocol;
import com.squareup.okhttp.Request;
import com.squareup.okhttp.RequestBody;
import com.squareup.okhttp.Response;
import io.grpc.ManagedChannel;
import io.grpc.ManagedChannelBuilder;
import java.io.IOException;
import java.util.Arrays;

public class TextileConfig implements Config {
    public static String scheme = "https";
    public static String host = "api.textile.io";
    public static int port = 6446;
    public static String session;

    public static ManagedChannel channel;
    public String cloud = "https://cloud.textile.io:443/register";

    private static String token;
    private static String deviceId;

    /**
     * TextileConfig provide auth wrappers to use hosted Thread services from Textile
     * @param token your Project Token from Textile CLI
     * @param deviceId a uuid for this app install.
     */
    public TextileConfig(String token, String deviceId) {
        this.deviceId = deviceId;
        this.token = token;
    }
    /**
     * @return the current session information
     */
    public String getSession(){
        return session;
    }

    /**
     *
     * @param session an existing session from getSession
     */
    public void setSession(String session){
        this.session = session;
    }
    /**
     * @return the gRPC managed channel
     */
    public ManagedChannel getChannel() {
        return channel;
    }
    /**
     * A private method to be used by the Client to initialize when instructed
     */
    public void init() throws Exception {
        if (scheme == "http") {
            channel = ManagedChannelBuilder
                    .forAddress(host, port)
                    .usePlaintext()
                    .build();
        } else {
            channel = ManagedChannelBuilder
                    .forAddress(host, port)
                    .useTransportSecurity()
                    .build();
        }
        if (this.session != null) {
            // skip the session refresh if it's not null
            return;
        }

        Response response = this.refreshSession();
        if (!response.isSuccessful()) throw new IOException(
                "Unexpected code " + response);
        String output = response.body().string();
        Gson gson = new Gson();
        Session result = gson.fromJson(output, Session.class);
        this.session = result.session_id;
        return;
    }
    public Response refreshSession() throws Exception {
        OkHttpClient client = new OkHttpClient();
        client.setProtocols(Arrays.asList(Protocol.HTTP_1_1));
        JsonObject json = new JsonObject();
        json.addProperty("token", token);
        json.addProperty("device_id",deviceId);
        String jsonString = json.toString();
        MediaType JSON = MediaType.parse("application/json; charset=utf-8");
        RequestBody body = RequestBody.create(JSON, jsonString);
        Request request = new Request.Builder()
                .url(cloud)
                .post(body)
                .build();
        return client.newCall(request).execute();
    }
}
