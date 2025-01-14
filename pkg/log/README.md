The log system is made as a package such that we can easily substitute it in case of need.
At the moment we are using logrus but in future we might need to switch to another log system, maybe for performance reasons.
- **Standard Library (`log`)**: Simple and sufficient for small to mid-sized projects.
- **logrus**: Feature-rich and highly extensible, good for systems where structured logging and third-party integrations are important.
- **zap**: Best for high-performance applications requiring fast, structured logging.
- **zerolog**: Efficient and zero-alloc logging, suitable for high-throughput systems.