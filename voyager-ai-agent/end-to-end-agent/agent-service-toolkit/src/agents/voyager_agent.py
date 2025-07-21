from datetime import datetime
from typing import Literal
import asyncio
import logging

from langchain_community.tools import DuckDuckGoSearchResults, OpenWeatherMapQueryRun
from langchain_community.utilities import OpenWeatherMapAPIWrapper
from langchain_core.language_models.chat_models import BaseChatModel
from langchain_core.messages import AIMessage, SystemMessage
from langchain_core.runnables import RunnableConfig, RunnableLambda, RunnableSerializable
from langchain_core.tools import BaseTool
from langgraph.graph import END, MessagesState, StateGraph
from langgraph.managed import RemainingSteps
from langgraph.prebuilt import ToolNode
from langgraph.graph.state import CompiledStateGraph

from agents.llama_guard import LlamaGuard, LlamaGuardOutput, SafetyAssessment
from agents.tools import calculator, get_weather
from core import get_model, settings

from langgraph.prebuilt import create_react_agent

import mcpservers.mcp_servers as mcpservers

logger = logging.getLogger(__name__)

model = get_model(settings.DEFAULT_MODEL)

class AgentState(MessagesState, total=False):
    """`total=False` is PEP589 specs.

    documentation: https://typing.readthedocs.io/en/latest/spec/typeddict.html#totality
    """

    safety: LlamaGuardOutput
    remaining_steps: RemainingSteps


web_search = DuckDuckGoSearchResults(name="WebSearch")
tools = [calculator, get_weather]
all_tools: list[BaseTool] = []






# Add weather tool if API key is set
# # Register for an API key at https://openweathermap.org/api/
# if settings.OPENWEATHERMAP_API_KEY:
#     wrapper = OpenWeatherMapAPIWrapper(
#         openweathermap_api_key=settings.OPENWEATHERMAP_API_KEY.get_secret_value()
#     )
#     tools.append(OpenWeatherMapQueryRun(name="Weather", api_wrapper=wrapper))

current_date = datetime.now().strftime("%B %d, %Y")
instructions = f"""
    You are a helpful assistant that can help the user with their database migration.
    You have access to the following tools:
    - calculator: to help with math questions
    - get_weather: to get the weather for any given city
    - voyager : interact with voyager tools
    - list_tables: list all the tables in the database.
    - read_query: execute and read-only SQL query on the database.
    For any of the tools pertaining to the database, ensure you use postgresql dialect.
    """


def wrap_model(model: BaseChatModel) -> RunnableSerializable[AgentState, AIMessage]:
    bound_model = model.bind_tools(all_tools)
    preprocessor = RunnableLambda(
        lambda state: [SystemMessage(content=instructions)] + state["messages"],
        name="StateModifier",
    )
    return preprocessor | bound_model  # type: ignore[return-value]


def format_safety_message(safety: LlamaGuardOutput) -> AIMessage:
    content = (
        f"This conversation was flagged for unsafe content: {', '.join(safety.unsafe_categories)}"
    )
    return AIMessage(content=content)


async def acall_model(state: AgentState, config: RunnableConfig) -> AgentState:
    m = get_model(config["configurable"].get("model", settings.DEFAULT_MODEL))
    model_runnable = wrap_model(m)
    response = await model_runnable.ainvoke(state, config)

    # import pdb; pdb.set_trace()

    # Run llama guard check here to avoid returning the message if it's unsafe
    llama_guard = LlamaGuard()
    safety_output = await llama_guard.ainvoke("Agent", state["messages"] + [response])
    if safety_output.safety_assessment == SafetyAssessment.UNSAFE:
        return {"messages": [format_safety_message(safety_output)], "safety": safety_output}

    if state["remaining_steps"] < 2 and response.tool_calls:
        return {
            "messages": [
                AIMessage(
                    id=response.id,
                    content="Sorry, need more steps to process this request.",
                )
            ]
        }
    # We return a list, because this will get added to the existing list
    print(f"\nResponse: {response}\n")
    return {"messages": [response]}


async def llama_guard_input(state: AgentState, config: RunnableConfig) -> AgentState:
    llama_guard = LlamaGuard()
    safety_output = await llama_guard.ainvoke("User", state["messages"])
    return {"safety": safety_output, "messages": []}


async def block_unsafe_content(state: AgentState, config: RunnableConfig) -> AgentState:
    safety: LlamaGuardOutput = state["safety"]
    return {"messages": [format_safety_message(safety)]}


# Define the graph
# Check for unsafe input and block further processing if found
def check_safety(state: AgentState) -> Literal["unsafe", "safe"]:
    safety: LlamaGuardOutput = state["safety"]
    match safety.safety_assessment:
        case SafetyAssessment.UNSAFE:
            return "unsafe"
        case _:
            return "safe"

# After "model", if there are tool calls, run "tools". Otherwise END.
def pending_tool_calls(state: AgentState) -> Literal["tools", "done"]:
    last_message = state["messages"][-1]
    if not isinstance(last_message, AIMessage):
        raise TypeError(f"Expected AIMessage, got {type(last_message)}")
    if last_message.tool_calls:
        return "tools"
    return "done"

async def compile_voyager_agent() -> CompiledStateGraph:
    await mcpservers.load_mcp_tools()
    global all_tools
    all_tools = tools + mcpservers.get_mcp_tools()
    print(f"Tools: {all_tools}")

    # agent = create_react_agent(
    #     model=model,
    #     tools=all_tools,  
    #     prompt=instructions  
    # )

    # return agent


    agent = StateGraph(AgentState)
    agent.add_node("model", acall_model)
    agent.add_node("tools", ToolNode(all_tools))
    agent.add_node("guard_input", llama_guard_input)
    agent.add_node("block_unsafe_content", block_unsafe_content)
    agent.set_entry_point("guard_input")

    agent.add_conditional_edges(
        "guard_input", check_safety, {"unsafe": "block_unsafe_content", "safe": "model"}
    )

    # Always END after blocking unsafe content
    agent.add_edge("block_unsafe_content", END)

    # Always run "model" after "tools"
    agent.add_edge("tools", "model")


    agent.add_conditional_edges("model", pending_tool_calls, {"tools": "tools", "done": END})

    voyager_agent = agent.compile()
    return voyager_agent

# def register_voyager_agent_with_mcp_tools_and_compile(mcp_tools: list[BaseTool]):
#     all_tools = tools + mcp_tools
#     agent.add_node("tools", ToolNode(all_tools))
#     return agent.compile()