package io.textile.threads;

import com.google.gson.Gson;
import com.google.gson.JsonObject;
import com.squareup.okhttp.Callback;
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
import java.util.function.Consumer;

@Deprecated
public class TextileConfig implements Config {
    public static String scheme = "https";
    public static String host = "api.textile.io";
    public static int port = 6446;
    public static String session;
    private static String token;
    private static String deviceId;
    public static ManagedChannel channel;
    public String cloud = "https://cloud.textile.io:443/register";

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
     * @return the gRPC managed channel
     */
    public ManagedChannel getChannel() {
        return channel;
    }
    /**
     * A private method to be used by the Client to initialize when instructed
     * @param ready
     */
    public void init(Consumer<Boolean> ready) {
        this.refreshSession((result) -> {
            this.session = result.session_id;
            ready.accept(true);
        });
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
    }
    public void refreshSession(Consumer<Session> callback) {
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
        client.newCall(request).enqueue(new Callback() {
            @Override
            public void onFailure(Request request, IOException throwable) {
                throwable.printStackTrace();
                System.out.println("Info: err " + throwable.getMessage());
            }

            @Override
            public void onResponse(Response response) throws IOException {
                if (!response.isSuccessful()) throw new IOException(
                        "Unexpected code " + response);
                String output = response.body().string();
                System.out.println("Info: response " + response.toString());
                System.out.println("Info: response " + output);

                Gson gson = new Gson();
                Session result = gson.fromJson(output, Session.class);
                System.out.println("Info: response " + result.session_id);
                callback.accept(result);
            }
        });
    }
}

