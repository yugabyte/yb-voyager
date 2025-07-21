from langchain_mcp_adapters.client import MultiServerMCPClient
from langchain_core.tools import BaseTool




mcp_client = MultiServerMCPClient(
    {
        "postgresql": {
            "command": "npx",
            "args": [
                "-y",
                "@executeautomation/database-server",
                "--postgresql",
                "--host",
                "127.0.0.1",
                "--database",
                "test",
                "--user",
                "amakala",
                "--password",
                "password"
            ],
            "transport": "stdio",
        },
        "voyager": {
            "command": "yb-voyager",
            "args": ["mcp-server"],
            "transport": "stdio",
        },
        # "exportDir": {
        #     "command": "npx",
        #     "args": [
        #         "-y",
        #         "@modelcontextprotocol/server-filesystem",
        #         "/Users/amakala/Documents/test/integ"
        #     ],
        #     "transport": "stdio",
        # },
        # "yugabyte": {
        #     "command": "npx",
        #     "args": [
        #         "-y",
        #         "@executeautomation/database-server",
        #         "--postgresql",
        #         "--host",
        #         "127.0.0.1",
        #         "--port",
        #         "5433",
        #         "--database",
        #         "test",
        #         "--user",
        #         "yugabyte",
        #         "--password",
        #         "yugabyte"
        #     ],
        #     "transport": "stdio",
        # }

    }
)


mcp_tools: list[BaseTool] = []

async def load_mcp_tools():
    global mcp_tools
    if len(mcp_tools) == 0:
        mcp_tools = await mcp_client.get_tools()


def get_mcp_tools():
    # if len(mcp_tools) == 0:
    #     raise Exception("MCP tools not loaded")
    return mcp_tools


# def load_and_get_mcp_tools_sync():
#     return asyncio.run(mcp_client.get_tools())