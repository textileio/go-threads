package io.textile.threads;

import io.grpc.ManagedChannel;
import io.grpc.ManagedChannelBuilder;

public class DefaultConfig implements Config {
    /**
     * http or https
     */
    public static String scheme = "http";
    /**
     * host IP or domain of threadsd
     */
    public static String host = "localhost";
    /**
     * port for calling threadsd api
     */
    public static int port = 6006;
    public static String session;
    public static ManagedChannel channel;

    /**
     * @return the current session information
     */
    public String getSession() {
        return session;
    }
    public void setSession(String session) { this.session = session; }
    /**
     * @return the gRPC managed channel
     */
    public ManagedChannel getChannel() {
        return channel;
    }

    /**
     * A private method to be used by the Client to initialize when instructed
     */
    public void init() {
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
        return;
    }
}
